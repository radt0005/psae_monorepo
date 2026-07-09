import os

import pytest

import spade.secrets as secrets_mod
from spade import get_secret


@pytest.fixture(autouse=True)
def reset_cache():
    secrets_mod._secrets = None
    yield
    secrets_mod._secrets = None
    os.environ.pop("SPADE_SECRETS", None)


def test_get_secret_returns_value(monkeypatch):
    monkeypatch.setenv("SPADE_SECRETS", '{"db": "postgres://user:pw@host/db"}')
    assert get_secret("db") == "postgres://user:pw@host/db"


def test_get_secret_missing_raises(monkeypatch):
    monkeypatch.setenv("SPADE_SECRETS", '{"db": "x"}')
    with pytest.raises(KeyError):
        get_secret("missing")


def test_env_scrubbed_after_load(monkeypatch):
    monkeypatch.setenv("SPADE_SECRETS", '{"db": "x"}')
    get_secret("db")
    assert "SPADE_SECRETS" not in os.environ


def test_no_secrets_env(monkeypatch):
    monkeypatch.delenv("SPADE_SECRETS", raising=False)
    with pytest.raises(KeyError):
        get_secret("db")
