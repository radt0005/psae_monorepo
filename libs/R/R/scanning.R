#' Load parameters from params.yaml
#'
#' Reads scalar parameters from the params.yaml file in the current working
#' directory. Returns an empty list if the file doesn't exist or is empty.
#'
#' @return A named list of parameters.
#' @keywords internal
load_params <- function() {
  params_path <- "params.yaml"
  if (!file.exists(params_path)) return(list())
  params <- yaml::read_yaml(params_path)
  if (is.null(params)) return(list())
  params
}

#' Scan input directories for typed file inputs
#'
#' Scans the `./inputs/` directory and builds typed arguments based on type
#' hints. Each subdirectory of `inputs/` corresponds to a named input parameter.
#'
#' @param type_hints A named list mapping parameter names to type name strings.
#' @return A named list of typed input objects.
#' @keywords internal
scan_inputs <- function(type_hints = list()) {
  inputs_dir <- "inputs"
  if (!dir.exists(inputs_dir)) return(list())

  result <- list()
  subdirs <- sort(list.dirs(inputs_dir, recursive = FALSE, full.names = TRUE))

  for (subdir in subdirs) {
    name <- basename(subdir)
    expected_type <- type_hints[[name]]
    files <- sort(list.files(subdir, full.names = TRUE, no.. = TRUE))
    files <- files[!file.info(files)$isdir]

    if (!is.null(expected_type)) {
      # Directory type
      if (expected_type %in% c("Directory")) {
        result[[name]] <- Directory(path = subdir)
        next
      }
      # Collection types
      collection_types <- c("FileCollection", "RasterFileCollection",
                            "VectorFileCollection", "TabularFileCollection")
      if (expected_type %in% collection_types) {
        constructor <- match.fun(expected_type)
        result[[name]] <- constructor(paths = files)
        next
      }
      # File types
      file_types <- c("File", "RasterFile", "VectorFile", "TabularFile", "JsonFile")
      if (expected_type %in% file_types) {
        if (length(files) == 0) {
          stop(sprintf("Input '%s' expects a file but directory '%s' is empty", name, subdir))
        }
        constructor <- match.fun(expected_type)
        result[[name]] <- constructor(path = files[1])
        next
      }
    }

    # Default: infer from file count
    if (length(files) == 0) {
      stop(sprintf("Input directory '%s' is empty", subdir))
    }
    if (length(files) == 1) {
      result[[name]] <- File(path = files[1])
    } else {
      result[[name]] <- FileCollection(paths = files)
    }
  }

  result
}

#' Build function arguments from params and inputs
#'
#' Merges parameters from params.yaml with scanned file inputs. Inputs take
#' precedence over params when names collide.
#'
#' @param fn A handler function with optional spade type hints.
#' @return A named list of arguments ready for do.call().
#' @keywords internal
build_function_args <- function(fn) {
  type_hints <- get_type_hints(fn)
  params <- load_params()
  inputs <- scan_inputs(type_hints)

  # Merge: inputs take precedence over params
  args <- params
  for (name in names(inputs)) {
    args[[name]] <- inputs[[name]]
  }
  args
}
