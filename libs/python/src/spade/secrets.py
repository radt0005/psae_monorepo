"""Access to secrets injected into a block by the Spade runtime.

Secrets are delivered as the ``SPADE_SECRETS`` environment variable — a JSON
object mapping the block's logical secret names to their values — by the worker
(cloud) or the CLI (local). This module parses that blob, serves values through
:func:`get_secret`, and scrubs the variable from the environment so it is not
inherited by any subprocess the block spawns. See ``spec/secrets.md`` §4.
"""

import json
import os
from typing import Dict, Optional

_secrets: Optional[Dict[str, str]] = None


def _parse_secrets(raw: Optional[str]) -> Dict[str, str]:
    if not raw:
        return {}
    return json.loads(raw)


def _load_secrets() -> Dict[str, str]:
    """Parse and cache ``SPADE_SECRETS``, removing it from the environment.

    The variable is popped on first read so it is not inherited by subprocesses
    the block spawns. Idempotent: subsequent calls return the cached mapping.
    """
    global _secrets
    if _secrets is None:
        _secrets = _parse_secrets(os.environ.pop("SPADE_SECRETS", None))
    return _secrets


def get_secret(name: str) -> str:
    """Return the secret bound to a logical ``name`` for this block.

    The mapping from logical name to a stored secret is declared in the
    pipeline (``spec/secrets.md`` §3.2); the value is injected by the worker
    (cloud) or CLI (local). Raises :class:`KeyError` if the name was not
    provided — a declared-but-unresolved secret is a real error, not empty.
    """
    secrets = _load_secrets()
    try:
        return secrets[name]
    except KeyError:
        raise KeyError(
            f"secret {name!r} was not provided to this block; "
            "declare it in the pipeline's 'secrets' mapping"
        ) from None
