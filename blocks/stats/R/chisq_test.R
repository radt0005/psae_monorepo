# Block: stats.chisq_test
#
# Pearson's chi-squared test of independence between two categorical columns.
library(spade)

handler <- function(data, column, by, correct = TRUE) {
  df <- read.csv(data@path, stringsAsFactors = FALSE, check.names = FALSE)

  if (!nzchar(column) || !nzchar(by)) {
    stop("'column' and 'by' parameters are required")
  }
  for (col in c(column, by)) {
    if (!col %in% names(df)) {
      stop(sprintf("column '%s' not found in data", col))
    }
  }

  tab <- table(df[[column]], df[[by]])
  ht <- suppressWarnings(stats::chisq.test(tab, correct = correct))

  # Represent the observed table as { column_level: { by_level: count } }.
  observed <- lapply(seq_len(nrow(tab)), function(i) {
    stats::setNames(as.list(as.integer(tab[i, ])), colnames(tab))
  })
  names(observed) <- rownames(tab)

  result <- list(
    method = ht$method,
    column = column,
    by = by,
    statistic = unname(ht$statistic),
    df = unname(ht$parameter),
    p_value = unname(ht$p.value),
    observed = observed
  )

  out_path <- "chisq_test.json"
  jsonlite::write_json(result, out_path, auto_unbox = TRUE, pretty = TRUE, digits = NA, na = "null")
  JsonFile(out_path)
}
spade_types(handler) <- list(
  data = "TabularFile", column = "character", by = "character",
  correct = "logical", .return = "JsonFile"
)
attr(handler, "spade_description") <- "Chi-squared test of independence between two categorical columns."

run(handler)
