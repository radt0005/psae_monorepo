test_that("build() simple function with File type hint", {
  handler <- function(input) NULL
  spade_types(handler) <- list(input = "File")

  manifest <- build(handler)
  expect_equal(manifest$inputs$input$type, "file")
})

test_that("build() typed inputs with format fields", {
  handler <- function(raster, vector) NULL
  spade_types(handler) <- list(raster = "RasterFile", vector = "VectorFile")

  manifest <- build(handler)
  expect_equal(manifest$inputs$raster$type, "file")
  expect_equal(manifest$inputs$raster$format, "GeoTIFF")
  expect_equal(manifest$inputs$vector$type, "file")
  expect_equal(manifest$inputs$vector$format, "GeoJSON")
})

test_that("build() scalar input types", {
  handler <- function(name, count, flag) NULL
  spade_types(handler) <- list(name = "character", count = "integer", flag = "logical")

  manifest <- build(handler)
  expect_equal(manifest$inputs$name$type, "string")
  expect_equal(manifest$inputs$count$type, "number")
  expect_equal(manifest$inputs$flag$type, "boolean")
})

test_that("build() with return type hint", {
  handler <- function(source) NULL
  spade_types(handler) <- list(source = "RasterFile", .return = "RasterFile")

  manifest <- build(handler)
  expect_equal(manifest$outputs$raster$type, "file")
  expect_equal(manifest$outputs$raster$format, "GeoTIFF")
})

test_that("build() with description attribute", {
  handler <- function(source) NULL
  spade_types(handler) <- list(source = "File")
  attr(handler, "spade_description") <- "Processes input files"

  manifest <- build(handler)
  expect_equal(manifest$description, "Processes input files")
})

test_that("build() no type hints returns empty inputs/outputs", {
  handler <- function(x, y) NULL

  manifest <- build(handler)
  expect_equal(length(manifest$inputs), 0)
  expect_equal(length(manifest$outputs), 0)
})

test_that("build() collection input type", {
  handler <- function(tiles) NULL
  spade_types(handler) <- list(tiles = "RasterFileCollection")

  manifest <- build(handler)
  expect_equal(manifest$inputs$tiles$type, "collection")
  expect_equal(manifest$inputs$tiles$item_type, "file")
  expect_equal(manifest$inputs$tiles$format, "GeoTIFF")
})

test_that("build() no return type gives empty outputs", {
  handler <- function(source) NULL
  spade_types(handler) <- list(source = "File")

  manifest <- build(handler)
  expect_equal(length(manifest$outputs), 0)
})

test_that("build() numeric type maps to number", {
  handler <- function(value) NULL
  spade_types(handler) <- list(value = "numeric")

  manifest <- build(handler)
  expect_equal(manifest$inputs$value$type, "number")
})

test_that("build() mixed file and scalar inputs", {
  handler <- function(source, buffer, method) NULL
  spade_types(handler) <- list(
    source = "RasterFile",
    buffer = "numeric",
    method = "character",
    .return = "RasterFile"
  )

  manifest <- build(handler)
  expect_equal(manifest$inputs$source$type, "file")
  expect_equal(manifest$inputs$source$format, "GeoTIFF")
  expect_equal(manifest$inputs$buffer$type, "number")
  expect_equal(manifest$inputs$method$type, "string")
  expect_equal(manifest$outputs$raster$type, "file")
})

test_that("build() TabularFile input", {
  handler <- function(data) NULL
  spade_types(handler) <- list(data = "TabularFile")

  manifest <- build(handler)
  expect_equal(manifest$inputs$data$type, "file")
  expect_equal(manifest$inputs$data$format, "CSV")
})

test_that("build() JsonFile input", {
  handler <- function(config) NULL
  spade_types(handler) <- list(config = "JsonFile")

  manifest <- build(handler)
  expect_equal(manifest$inputs$config$type, "json")
})

test_that("build() Directory input", {
  handler <- function(dir) NULL
  spade_types(handler) <- list(dir = "Directory")

  manifest <- build(handler)
  expect_equal(manifest$inputs$dir$type, "directory")
})

test_that("build() return type FileCollection", {
  handler <- function(source) NULL
  spade_types(handler) <- list(source = "File", .return = "FileCollection")

  manifest <- build(handler)
  expect_equal(manifest$outputs$files$type, "collection")
  expect_equal(manifest$outputs$files$item_type, "file")
})

test_that("build() return type VectorFile", {
  handler <- function(source) NULL
  spade_types(handler) <- list(source = "File", .return = "VectorFile")

  manifest <- build(handler)
  expect_equal(manifest$outputs$vector$type, "file")
  expect_equal(manifest$outputs$vector$format, "GeoJSON")
})
