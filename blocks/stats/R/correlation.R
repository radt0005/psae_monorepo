# Block: stats.correlation
#
# Correlation matrix over the numeric columns of a tabular dataset.
library(spade)

parse_columns <- function(columns) {
  if (is.null(columns) || !nzchar(columns)) {
    return(character(0))
  }
  parts <- trimws(strsplit(columns, ",", fixed = TRUE)[[1]])
  parts[nzchar(parts)]
}

handler <- function(data, method = "pearson", columns = "") {
  if (!nzchar(method)) {
    method <- "pearson"
  }
  method <- match.arg(method, c("pearson", "spearman", "kendall"))

  df <- read.csv(data@path, stringsAsFactors = FALSE, check.names = FALSE)

  selected <- parse_columns(columns)
  if (length(selected) == 0) {
    selected <- names(df)[vapply(df, is.numeric, logical(1))]
  } else {
    missing <- setdiff(selected, names(df))
    if (length(missing) > 0) {
      stop(sprintf("columns not found in data: %s", paste(missing, collapse = ", ")))
    }
  }
  if (length(selected) < 2) {
    stop("correlation requires at least two numeric columns")
  }

  mat <- stats::cor(df[selected], method = method, use = "pairwise.complete.obs")

  # Represent the matrix as a named list of named lists so it serializes to a
  # readable nested JSON object keyed by column name.
  matrix_obj <- lapply(selected, function(row) as.list(mat[row, ]))
  names(matrix_obj) <- selected

  result <- list(method = method, columns = selected, matrix = matrix_obj)

  out_path <- "correlation.json"
  jsonlite::write_json(result, out_path, auto_unbox = TRUE, pretty = TRUE, digits = NA, na = "null")
  JsonFile(out_path)
}
spade_types(handler) <- list(
  data = "TabularFile", method = "character", columns = "character", .return = "JsonFile"
)
attr(handler, "spade_description") <- "Correlation matrix over numeric columns of a tabular dataset."

run(handler)
