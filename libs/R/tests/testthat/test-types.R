test_that("File creation with valid path", {
  f <- File(path = "/tmp/test.tif")
  expect_s4_class(f, "File")
  expect_equal(f@path, "/tmp/test.tif")
})

test_that("File slot access via @path", {
  f <- File(path = "/data/raster.tif")
  expect_equal(f@path, "/data/raster.tif")
})

test_that("Directory creation and slot access", {
  d <- Directory(path = "/tmp/mydir")
  expect_s4_class(d, "Directory")
  expect_equal(d@path, "/tmp/mydir")
})

test_that("RasterFile inherits from File", {
  rf <- RasterFile(path = "/tmp/raster.tif")
  expect_s4_class(rf, "RasterFile")
  expect_true(is(rf, "File"))
  expect_equal(rf@path, "/tmp/raster.tif")
})

test_that("VectorFile inherits from File", {
  vf <- VectorFile(path = "/tmp/vector.geojson")
  expect_s4_class(vf, "VectorFile")
  expect_true(is(vf, "File"))
})

test_that("TabularFile inherits from File", {
  tf <- TabularFile(path = "/tmp/data.csv")
  expect_s4_class(tf, "TabularFile")
  expect_true(is(tf, "File"))
})

test_that("JsonFile inherits from File", {
  jf <- JsonFile(path = "/tmp/config.json")
  expect_s4_class(jf, "JsonFile")
  expect_true(is(jf, "File"))
})

test_that("FileCollection creation with character vector", {
  fc <- FileCollection(paths = c("/tmp/a.tif", "/tmp/b.tif"))
  expect_s4_class(fc, "FileCollection")
  expect_equal(fc@paths, c("/tmp/a.tif", "/tmp/b.tif"))
})

test_that("RasterFileCollection inherits from FileCollection", {
  rfc <- RasterFileCollection(paths = c("/tmp/a.tif", "/tmp/b.tif"))
  expect_s4_class(rfc, "RasterFileCollection")
  expect_true(is(rfc, "FileCollection"))
})

test_that("VectorFileCollection inherits from FileCollection", {
  vfc <- VectorFileCollection(paths = c("/tmp/a.geojson", "/tmp/b.geojson"))
  expect_s4_class(vfc, "VectorFileCollection")
  expect_true(is(vfc, "FileCollection"))
})

test_that("TabularFileCollection inherits from FileCollection", {
  tfc <- TabularFileCollection(paths = c("/tmp/a.csv", "/tmp/b.csv"))
  expect_s4_class(tfc, "TabularFileCollection")
  expect_true(is(tfc, "FileCollection"))
})

test_that("Empty collection is valid", {
  fc <- FileCollection(paths = character(0))
  expect_s4_class(fc, "FileCollection")
  expect_equal(length(fc@paths), 0)
})

test_that("File validity check rejects empty path", {
  expect_error(File(path = ""), "path must be a non-empty single character string")
})

test_that("show() method for File prints readable output", {
  f <- File(path = "/tmp/test.tif")
  output <- capture.output(show(f))
  expect_match(output, "<File> /tmp/test.tif")
})

test_that("show() method for FileCollection prints readable output", {
  fc <- FileCollection(paths = c("/tmp/a.tif", "/tmp/b.tif"))
  output <- capture.output(show(fc))
  expect_match(output, "<FileCollection> \\[2 files\\]")
})

test_that("show() method for Directory prints readable output", {
  d <- Directory(path = "/tmp/mydir")
  output <- capture.output(show(d))
  expect_match(output, "<Directory> /tmp/mydir")
})

test_that("show() method for RasterFile subclass prints class name", {
  rf <- RasterFile(path = "/tmp/raster.tif")
  output <- capture.output(show(rf))
  expect_match(output, "<RasterFile> /tmp/raster.tif")
})

test_that("Directory validity check rejects empty path", {
  expect_error(Directory(path = ""), "path must be a non-empty single character string")
})
