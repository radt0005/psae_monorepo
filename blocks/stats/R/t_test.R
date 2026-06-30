# Block: stats.t_test
#
# One- or two-sample Student's t-test on a numeric column.
library(spade)

# Convert an htest object into a plain list suitable for JSON serialization.
htest_to_list <- function(ht) {
  ci <- if (!is.null(ht$conf.int)) as.numeric(ht$conf.int) else NULL
  list(
    method = ht$method,
    statistic = unname(ht$statistic),
    parameter = unname(ht$parameter),
    p_value = unname(ht$p.value),
    conf_int = ci,
    conf_level = if (!is.null(ci)) attr(ht$conf.int, "conf.level") else NULL,
    estimate = as.list(ht$estimate),
    alternative = ht$alternative
  )
}

handler <- function(data, value_column, group_column = "", mu = 0,
                    alternative = "two.sided", paired = FALSE, conf_level = 0.95) {
  if (!nzchar(alternative)) {
    alternative <- "two.sided"
  }
  alternative <- match.arg(alternative, c("two.sided", "less", "greater"))

  df <- read.csv(data@path, stringsAsFactors = FALSE, check.names = FALSE)
  if (!nzchar(value_column)) {
    stop("'value_column' parameter is required")
  }
  if (!value_column %in% names(df)) {
    stop(sprintf("column '%s' not found in data", value_column))
  }
  values <- df[[value_column]]
  if (!is.numeric(values)) {
    stop(sprintf("column '%s' is not numeric", value_column))
  }

  if (nzchar(group_column)) {
    if (!group_column %in% names(df)) {
      stop(sprintf("column '%s' not found in data", group_column))
    }
    groups <- factor(df[[group_column]])
    if (nlevels(groups) != 2) {
      stop(sprintf(
        "two-sample t-test requires exactly two groups in '%s', found %d",
        group_column, nlevels(groups)
      ))
    }
    # Use the default (two-vector) method rather than the formula method: the
    # formula method rejects `paired` in R >= 4.4. Splitting by level also lets
    # us label the estimates with the group names.
    levs <- levels(groups)
    x <- values[groups == levs[1]]
    y <- values[groups == levs[2]]
    if (paired && length(x) != length(y)) {
      stop("paired two-sample t-test requires equal-length groups")
    }
    ht <- stats::t.test(
      x, y, alternative = alternative,
      paired = paired, conf.level = conf_level
    )
    names(ht$estimate) <- if (paired) "mean of differences" else paste0("mean: ", levs)
  } else {
    ht <- stats::t.test(
      values, mu = mu, alternative = alternative, conf.level = conf_level
    )
  }

  result <- htest_to_list(ht)
  out_path <- "t_test.json"
  jsonlite::write_json(result, out_path, auto_unbox = TRUE, pretty = TRUE, digits = NA, na = "null")
  JsonFile(out_path)
}
spade_types(handler) <- list(
  data = "TabularFile", value_column = "character", group_column = "character",
  mu = "numeric", alternative = "character", paired = "logical",
  conf_level = "numeric", .return = "JsonFile"
)
attr(handler, "spade_description") <- "One- or two-sample Student's t-test on a numeric column."

run(handler)
