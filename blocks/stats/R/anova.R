# Block: stats.anova
#
# One-way analysis of variance of a numeric response across a grouping column.
library(spade)

handler <- function(data, value_column, group_column) {
  df <- read.csv(data@path, stringsAsFactors = FALSE, check.names = FALSE)

  if (!nzchar(value_column) || !nzchar(group_column)) {
    stop("'value_column' and 'group_column' parameters are required")
  }
  for (col in c(value_column, group_column)) {
    if (!col %in% names(df)) {
      stop(sprintf("column '%s' not found in data", col))
    }
  }
  values <- df[[value_column]]
  if (!is.numeric(values)) {
    stop(sprintf("column '%s' is not numeric", value_column))
  }
  groups <- factor(df[[group_column]])
  if (nlevels(groups) < 2) {
    stop(sprintf("ANOVA requires at least two groups in '%s'", group_column))
  }

  fit <- stats::aov(values ~ groups)
  tab <- summary(fit)[[1]]

  # The aov summary table has a row per term ("groups") plus "Residuals".
  term_names <- trimws(rownames(tab))
  to_row <- function(i) {
    list(
      df = tab[i, "Df"],
      sum_sq = tab[i, "Sum Sq"],
      mean_sq = tab[i, "Mean Sq"],
      f_value = if ("F value" %in% colnames(tab)) tab[i, "F value"] else NULL,
      p_value = if ("Pr(>F)" %in% colnames(tab)) tab[i, "Pr(>F)"] else NULL
    )
  }
  rows <- lapply(seq_len(nrow(tab)), to_row)
  names(rows) <- ifelse(term_names == "groups", group_column, term_names)

  result <- list(
    value_column = value_column,
    group_column = group_column,
    groups = levels(groups),
    table = rows
  )

  out_path <- "anova.json"
  jsonlite::write_json(result, out_path, auto_unbox = TRUE, pretty = TRUE, digits = NA, na = "null")
  JsonFile(out_path)
}
spade_types(handler) <- list(
  data = "TabularFile", value_column = "character", group_column = "character",
  .return = "JsonFile"
)
attr(handler, "spade_description") <- "One-way ANOVA of a numeric response across a grouping column."

run(handler)
