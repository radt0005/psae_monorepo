test_that("load_params reads basic params.yaml", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  write_params(list(buffer_distance = 30, method = "bilinear"))
  params <- load_params()
  expect_equal(params$buffer_distance, 30)
  expect_equal(params$method, "bilinear")
})

test_that("load_params returns empty list for empty file", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  writeLines("", "params.yaml")
  params <- load_params()
  expect_equal(params, list())
})

test_that("load_params returns empty list for missing file", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  params <- load_params()
  expect_equal(params, list())
})

test_that("scan_inputs with typed single file", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  create_input_file("raster", "data.tif")
  result <- scan_inputs(list(raster = "RasterFile"))
  expect_s4_class(result$raster, "RasterFile")
  expect_true(grepl("data.tif$", result$raster@path))
})

test_that("scan_inputs untyped single file defaults to File", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  create_input_file("source", "data.tif")
  result <- scan_inputs()
  expect_s4_class(result$source, "File")
})

test_that("scan_inputs with directory type", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  create_input_file("source", "data.tif")
  result <- scan_inputs(list(source = "Directory"))
  expect_s4_class(result$source, "Directory")
  expect_true(grepl("inputs/source$", result$source@path))
})

test_that("scan_inputs with collection type", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  create_input_collection("tiles", c("a.tif", "b.tif", "c.tif"))
  result <- scan_inputs(list(tiles = "RasterFileCollection"))
  expect_s4_class(result$tiles, "RasterFileCollection")
  expect_equal(length(result$tiles@paths), 3)
})

test_that("scan_inputs discovers multiple inputs", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  create_input_file("alpha", "a.tif")
  create_input_file("beta", "b.csv")
  result <- scan_inputs()
  expect_true("alpha" %in% names(result))
  expect_true("beta" %in% names(result))
})

test_that("scan_inputs errors on empty input directory", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  dir.create("inputs/empty_dir")
  expect_error(scan_inputs(), "empty")
})

test_that("scan_inputs untyped multiple files defaults to FileCollection", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  create_input_collection("source", c("a.tif", "b.tif"))
  result <- scan_inputs()
  expect_s4_class(result$source, "FileCollection")
  expect_equal(length(result$source@paths), 2)
})

test_that("scan_inputs returns empty list when inputs dir missing", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  unlink("inputs", recursive = TRUE)
  result <- scan_inputs()
  expect_equal(result, list())
})

test_that("build_function_args merges params and inputs", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  write_params(list(buffer = 30))
  create_input_file("source", "data.tif")

  handler <- function(source, buffer) NULL
  args <- build_function_args(handler)
  expect_equal(args$buffer, 30)
  expect_s4_class(args$source, "File")
})

test_that("build_function_args inputs take precedence over params", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  write_params(list(source = "param_value"))
  create_input_file("source", "data.tif")

  handler <- function(source) NULL
  args <- build_function_args(handler)
  # Input should win over param
  expect_s4_class(args$source, "File")
})
