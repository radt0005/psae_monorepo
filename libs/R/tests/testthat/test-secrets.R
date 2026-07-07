test_that("get_secret returns the injected value", {
  .spade_secrets$cache <- NULL
  Sys.setenv(SPADE_SECRETS = '{"db":"postgres://user:pw@host/db"}')
  expect_equal(get_secret("db"), "postgres://user:pw@host/db")
})

test_that("get_secret errors on a missing name", {
  .spade_secrets$cache <- NULL
  Sys.setenv(SPADE_SECRETS = '{"db":"x"}')
  expect_error(get_secret("nope"))
})

test_that("SPADE_SECRETS is scrubbed from the environment after load", {
  .spade_secrets$cache <- NULL
  Sys.setenv(SPADE_SECRETS = '{"db":"x"}')
  get_secret("db")
  expect_identical(Sys.getenv("SPADE_SECRETS", unset = "<unset>"), "<unset>")
})

test_that("get_secret errors when no secrets were provided", {
  .spade_secrets$cache <- NULL
  Sys.unsetenv("SPADE_SECRETS")
  expect_error(get_secret("db"))
})
