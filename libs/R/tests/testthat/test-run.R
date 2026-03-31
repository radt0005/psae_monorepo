test_that("run() calls simple handler with File input", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  create_input_file("file", "data.tif")

  called_with <- NULL
  handler <- function(file) {
    called_with <<- file
    NULL
  }

  run(handler)
  expect_s4_class(called_with, "File")
  expect_true(grepl("data.tif$", called_with@path))
})

test_that("run() passes params and inputs to handler", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  write_params(list(buffer = 30))
  create_input_file("source", "data.tif")

  received <- NULL
  handler <- function(source, buffer) {
    received <<- list(source = source, buffer = buffer)
    NULL
  }

  run(handler)
  expect_s4_class(received$source, "File")
  expect_equal(received$buffer, 30)
})

test_that("run() with typed inputs delivers correct types", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  create_input_file("source", "data.tif")

  received <- NULL
  handler <- function(source) {
    received <<- source
    NULL
  }
  spade_types(handler) <- list(source = "RasterFile")

  run(handler)
  expect_s4_class(received, "RasterFile")
})

test_that("run() writes handler output to outputs/", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  create_input_file("source", "data.tif")

  handler <- function(source) {
    result_path <- tempfile(fileext = ".tif")
    writeBin(charToRaw("processed"), result_path)
    RasterFile(path = result_path)
  }
  spade_types(handler) <- list(source = "RasterFile", .return = "RasterFile")

  run(handler)
  expect_true(dir.exists("outputs/raster"))
  output_files <- list.files("outputs/raster")
  expect_equal(length(output_files), 1)
})

test_that("run() writes dict output with multiple outputs", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  create_input_file("source", "data.tif")

  handler <- function(source) {
    r_path <- tempfile(fileext = ".tif")
    v_path <- tempfile(fileext = ".geojson")
    writeBin(charToRaw("raster"), r_path)
    writeBin(charToRaw("vector"), v_path)
    list(
      raster_out = RasterFile(path = r_path),
      vector_out = VectorFile(path = v_path)
    )
  }

  run(handler)
  expect_true(dir.exists("outputs/raster_out"))
  expect_true(dir.exists("outputs/vector_out"))
})

test_that("run() propagates handler errors", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  handler <- function() {
    stop("handler failed")
  }

  expect_error(run(handler), "handler failed")
})

test_that("run() with no return produces no output files", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  handler <- function() {
    invisible(NULL)
  }

  run(handler)
  output_subdirs <- list.dirs("outputs", recursive = FALSE)
  expect_equal(length(output_subdirs), 0)
})

test_that("run() filters extra params not in handler signature", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  write_params(list(buffer = 30, extra_param = "not_used"))

  received <- NULL
  handler <- function(buffer) {
    received <<- buffer
    NULL
  }

  run(handler)
  expect_equal(received, 30)
})

test_that("run() full workflow end-to-end", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  write_params(list(buffer_distance = 100))
  create_input_file("source", "input.tif", charToRaw("source raster"))
  create_input_collection("tiles", c("a.tif", "b.tif"), charToRaw("tile"))

  handler <- function(source, tiles, buffer_distance) {
    result_path <- tempfile(fileext = ".tif")
    # Simulate processing: combine input content
    writeBin(charToRaw(paste("processed with buffer", buffer_distance)), result_path)
    RasterFile(path = result_path)
  }
  spade_types(handler) <- list(
    source = "RasterFile",
    tiles = "RasterFileCollection",
    buffer_distance = "numeric",
    .return = "RasterFile"
  )

  run(handler)

  # Verify outputs
  expect_true(dir.exists("outputs/raster"))
  output_files <- list.files("outputs/raster")
  expect_equal(length(output_files), 1)
  output_content <- readBin(file.path("outputs/raster", output_files[1]), raw(), 1000)
  expect_match(rawToChar(output_content), "processed with buffer 100")
})

test_that("run() passes all args to handler with ...", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  write_params(list(buffer = 30, extra = "value"))

  received <- NULL
  handler <- function(...) {
    received <<- list(...)
    NULL
  }

  run(handler)
  expect_equal(received$buffer, 30)
  expect_equal(received$extra, "value")
})
