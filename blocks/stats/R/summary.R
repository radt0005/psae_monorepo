# Block: stats.summary
#
# Descriptive statistics for the numeric columns of a tabular dataset. Reads a
# CSV, computes summary statistics per numeric column, and writes them as JSON.
library(spade)

# Split a comma-separated parameter string into trimmed, non-empty names.
parse_columns <- function(columns) {
  if (is.null(columns) || !nzchar(columns)) {
    return(character(0))
  }
  parts <- trimws(strsplit(columns, ",", fixed = TRUE)[[1]])
  parts[nzchar(parts)]
}

handler <- function(data, columns = "") {
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

  stats <- list()
  for (col in selected) {
    values <- df[[col]]
    if (!is.numeric(values)) {
      stop(sprintf("column '%s' is not numeric", col))
    }
    finite <- values[!is.na(values)]
    quantiles <- stats::quantile(finite, probs = c(0.25, 0.5, 0.75), names = FALSE)
    stats[[col]] <- list(
      n = length(finite),
      missing = sum(is.na(values)),
      mean = mean(finite),
      sd = stats::sd(finite),
      min = min(finite),
      q25 = quantiles[1],
      median = quantiles[2],
      q75 = quantiles[3],
      max = max(finite)
    )
  }

  out_path <- "summary.json"
  jsonlite::write_json(stats, out_path, auto_unbox = TRUE, pretty = TRUE, digits = NA, na = "null")
  JsonFile(out_path)
}
spade_types(handler) <- list(data = "TabularFile", columns = "character", .return = "JsonFile")
attr(handler, "spade_description") <- "Per-column descriptive statistics for a tabular dataset."

run(handler)
