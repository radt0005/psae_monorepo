# Block: stats.frequency
#
# Frequency table for a categorical column, optionally cross-tabulated by a
# second column.
library(spade)

handler <- function(data, column, by = "") {
  df <- read.csv(data@path, stringsAsFactors = FALSE, check.names = FALSE)

  if (!nzchar(column)) {
    stop("'column' parameter is required")
  }
  if (!column %in% names(df)) {
    stop(sprintf("column '%s' not found in data", column))
  }

  has_by <- nzchar(by)
  if (has_by && !by %in% names(df)) {
    stop(sprintf("column '%s' not found in data", by))
  }

  if (!has_by) {
    # One-way frequency table: counts and proportions per level.
    tab <- table(df[[column]], useNA = "ifany")
    levels_chr <- names(tab)
    levels_chr[is.na(levels_chr)] <- "NA"
    counts <- as.integer(tab)
    proportions <- counts / sum(counts)
    result <- list(
      column = column,
      total = sum(counts),
      counts = stats::setNames(as.list(counts), levels_chr),
      proportions = stats::setNames(as.list(proportions), levels_chr)
    )
  } else {
    # Contingency table nested as { column_level: { by_level: count } }.
    tab <- table(df[[column]], df[[by]], useNA = "ifany")
    row_names <- rownames(tab); row_names[is.na(row_names)] <- "NA"
    col_names <- colnames(tab); col_names[is.na(col_names)] <- "NA"
    nested <- lapply(seq_len(nrow(tab)), function(i) {
      stats::setNames(as.list(as.integer(tab[i, ])), col_names)
    })
    names(nested) <- row_names
    result <- list(
      column = column,
      by = by,
      total = sum(tab),
      table = nested
    )
  }

  out_path <- "frequency.json"
  jsonlite::write_json(result, out_path, auto_unbox = TRUE, pretty = TRUE, digits = NA, na = "null")
  JsonFile(out_path)
}
spade_types(handler) <- list(
  data = "TabularFile", column = "character", by = "character", .return = "JsonFile"
)
attr(handler, "spade_description") <- "Frequency / contingency table for categorical columns."

run(handler)
