test_that("spade_types() returns NULL for unannotated function", {
  fn <- function(x) NULL
  expect_null(spade_types(fn))
})

test_that("spade_types<- sets type annotations", {
  fn <- function(source, buffer) NULL
  spade_types(fn) <- list(source = "RasterFile", buffer = "numeric", .return = "RasterFile")

  hints <- spade_types(fn)
  expect_equal(hints$source, "RasterFile")
  expect_equal(hints$buffer, "numeric")
  expect_equal(hints[[".return"]], "RasterFile")
})

test_that("spade_types<- rejects non-function", {
  expect_error(spade_types(42) <- list(x = "File"))
})

test_that("spade_types<- rejects non-list value", {
  fn <- function(x) NULL
  expect_error(spade_types(fn) <- "not a list")
})

test_that("get_type_hints excludes .return", {
  fn <- function(source) NULL
  spade_types(fn) <- list(source = "RasterFile", .return = "RasterFile")

  hints <- get_type_hints(fn)
  expect_equal(hints$source, "RasterFile")
  expect_null(hints[[".return"]])
})

test_that("get_type_hints returns empty list for unannotated", {
  fn <- function(x) NULL
  hints <- get_type_hints(fn)
  expect_equal(hints, list())
})

test_that("get_return_type extracts return type", {
  fn <- function(source) NULL
  spade_types(fn) <- list(source = "RasterFile", .return = "VectorFile")

  expect_equal(get_return_type(fn), "VectorFile")
})

test_that("get_return_type returns NULL when no return", {
  fn <- function(source) NULL
  spade_types(fn) <- list(source = "RasterFile")

  expect_null(get_return_type(fn))
})

test_that("get_return_type returns NULL for unannotated", {
  fn <- function(x) NULL
  expect_null(get_return_type(fn))
})
