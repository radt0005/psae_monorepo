import { test, expect, beforeEach } from "bun:test";
import { getSecret, _resetSecretsForTest } from "../src/secrets.ts";

beforeEach(() => {
  _resetSecretsForTest();
  delete process.env.SPADE_SECRETS;
});

test("returns injected secret value", () => {
  process.env.SPADE_SECRETS = JSON.stringify({ db: "postgres://user:pw@host/db" });
  expect(getSecret("db")).toBe("postgres://user:pw@host/db");
});

test("throws for a missing secret", () => {
  process.env.SPADE_SECRETS = JSON.stringify({ db: "x" });
  expect(() => getSecret("nope")).toThrow();
});

test("scrubs SPADE_SECRETS from the environment after load", () => {
  process.env.SPADE_SECRETS = JSON.stringify({ db: "x" });
  getSecret("db");
  expect(process.env.SPADE_SECRETS).toBeUndefined();
});

test("throws when no secrets were provided", () => {
  expect(() => getSecret("db")).toThrow();
});
