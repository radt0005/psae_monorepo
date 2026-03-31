#' Get type annotations from a function
#'
#' Retrieves the spade type annotations attached to a function via attributes.
#'
#' @param fn A function with spade type annotations.
#' @return A named list of type annotations, or NULL if none are set.
#' @export
spade_types <- function(fn) {
  attr(fn, "spade_types", exact = TRUE)
}

#' Set type annotations on a function
#'
#' Attaches spade type annotations to a function as an attribute.
#'
#' @param fn A function.
#' @param value A named list mapping parameter names to type name strings.
#'   The special key `.return` specifies the return type.
#' @return The function with type annotations attached.
#' @export
`spade_types<-` <- function(fn, value) {
  stopifnot(is.function(fn))
  stopifnot(is.list(value))
  attr(fn, "spade_types") <- value
  fn
}

#' Get parameter type hints (internal)
#'
#' Extracts type hints for parameters only (excludes .return).
#'
#' @param fn A function.
#' @return A named list of parameter type hints.
#' @keywords internal
get_type_hints <- function(fn) {
  hints <- spade_types(fn)
  if (is.null(hints)) return(list())
  hints[names(hints) != ".return"]
}

#' Get return type hint (internal)
#'
#' Extracts the return type string from a function's type hints.
#'
#' @param fn A function.
#' @return The return type string, or NULL.
#' @keywords internal
get_return_type <- function(fn) {
  hints <- spade_types(fn)
  if (is.null(hints)) return(NULL)
  hints[[".return"]]
}
