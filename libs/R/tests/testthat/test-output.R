test_that("infer_output_name for File", {
  expect_equal(infer_output_name(File(path = "/tmp/x")), "file")
})

test_that("infer_output_name for RasterFile", {
  expect_equal(infer_output_name(RasterFile(path = "/tmp/x")), "raster")
})

test_that("infer_output_name for VectorFile", {
  expect_equal(infer_output_name(VectorFile(path = "/tmp/x")), "vector")
})

test_that("infer_output_name for JsonFile", {
  expect_equal(infer_output_name(JsonFile(path = "/tmp/x")), "json")
})

test_that("infer_output_name for FileCollection", {
  expect_equal(infer_output_name(FileCollection(paths = "/tmp/x")), "files")
})

test_that("infer_output_name for RasterFileCollection", {
  expect_equal(infer_output_name(RasterFileCollection(paths = "/tmp/x")), "rasters")
})

test_that("write_outputs NULL result produces no output", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  write_outputs(NULL)
  # outputs dir should not exist (or be empty if pre-created)
  output_subdirs <- list.dirs("outputs", recursive = FALSE)
  expect_equal(length(output_subdirs), 0)
})

test_that("write_outputs single RasterFile writes to outputs/raster/", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  # Create a source file
  src <- tempfile(fileext = ".tif")
  writeBin(charToRaw("raster data"), src)

  result <- RasterFile(path = src)
  write_outputs(result)

  expect_true(dir.exists("outputs/raster"))
  output_files <- list.files("outputs/raster")
  expect_equal(length(output_files), 1)
})

test_that("write_outputs single file with manifest uses manifest name", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  src <- tempfile(fileext = ".tif")
  writeBin(charToRaw("raster data"), src)

  result <- RasterFile(path = src)
  manifest_outputs <- list(custom_output = list(type = "file"))
  write_outputs(result, manifest_outputs)

  expect_true(dir.exists("outputs/custom_output"))
})

test_that("write_outputs named list writes multiple subdirectories", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  src1 <- tempfile(fileext = ".tif")
  src2 <- tempfile(fileext = ".geojson")
  writeBin(charToRaw("raster"), src1)
  writeBin(charToRaw("vector"), src2)

  result <- list(
    raster_out = RasterFile(path = src1),
    vector_out = VectorFile(path = src2)
  )
  write_outputs(result)

  expect_true(dir.exists("outputs/raster_out"))
  expect_true(dir.exists("outputs/vector_out"))
})

test_that("write_outputs collection writes all files", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  src1 <- tempfile(tmpdir = tempdir(), fileext = ".tif")
  src2 <- tempfile(tmpdir = tempdir(), fileext = ".tif")
  writeBin(charToRaw("raster1"), src1)
  writeBin(charToRaw("raster2"), src2)

  result <- RasterFileCollection(paths = c(src1, src2))
  write_outputs(result)

  expect_true(dir.exists("outputs/rasters"))
  output_files <- list.files("outputs/rasters")
  expect_equal(length(output_files), 2)
})

test_that("write_outputs Directory copies directory contents", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  src_dir <- tempfile("dir_")
  dir.create(src_dir)
  writeBin(charToRaw("file1"), file.path(src_dir, "a.txt"))
  writeBin(charToRaw("file2"), file.path(src_dir, "b.txt"))

  result <- Directory(path = src_dir)
  write_outputs(result)

  expect_true(dir.exists("outputs/directory"))
  output_files <- list.files("outputs/directory")
  expect_true("a.txt" %in% output_files)
  expect_true("b.txt" %in% output_files)
})

test_that("write_outputs preserves filename", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  src <- tempfile(tmpdir = tempdir(), pattern = "original_name_", fileext = ".tif")
  writeBin(charToRaw("data"), src)

  result <- File(path = src)
  write_outputs(result)

  output_files <- list.files("outputs/file")
  expect_equal(output_files, basename(src))
})

test_that("read_block_manifest returns NULL when no manifest", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  result <- read_block_manifest()
  expect_null(result)
})

test_that("read_block_manifest reads block.yaml", {
  env <- setup_work_dir()
  on.exit(teardown_work_dir(env))

  yaml::write_yaml(
    list(outputs = list(raster = list(type = "file"))),
    "block.yaml"
  )

  result <- read_block_manifest()
  expect_equal(result$raster$type, "file")
})

test_that("read_block_manifest reads from env var path", {
  env <- setup_work_dir()
  on.exit({
    Sys.unsetenv("SPADE_BLOCK_MANIFEST")
    teardown_work_dir(env)
  })

  manifest_file <- tempfile(fileext = ".yaml")
  yaml::write_yaml(
    list(outputs = list(custom = list(type = "file"))),
    manifest_file
  )
  Sys.setenv(SPADE_BLOCK_MANIFEST = manifest_file)

  result <- read_block_manifest()
  expect_equal(result$custom$type, "file")
})

test_that("infer_output_name for TabularFile", {
  expect_equal(infer_output_name(TabularFile(path = "/tmp/x")), "tabular")
})

test_that("infer_output_name for Directory", {
  expect_equal(infer_output_name(Directory(path = "/tmp/x")), "directory")
})

test_that("infer_output_name for VectorFileCollection", {
  expect_equal(infer_output_name(VectorFileCollection(paths = "/tmp/x")), "vectors")
})

test_that("infer_output_name for TabularFileCollection", {
  expect_equal(infer_output_name(TabularFileCollection(paths = "/tmp/x")), "tables")
})
