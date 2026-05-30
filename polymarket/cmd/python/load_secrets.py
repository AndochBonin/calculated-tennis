"""Load credentials from AWS Secrets Manager into os.environ."""

from __future__ import annotations

import json
import os
import sys

SECRET_ID_ENV = "POLYMARKET_SECRETS_MANAGER_SECRET_ID"


def apply_secret_map(secret_map: dict[str, str]) -> None:
    """Set os.environ for each non-empty key/value (mirrors secrets/load.go)."""
    for key, value in secret_map.items():
        if not key or not key.strip():
            continue
        if value is None or not str(value).strip():
            continue
        os.environ[key] = str(value)


def load_from_secrets_manager(secret_id: str) -> None:
    secret_id = secret_id.strip()
    if not secret_id:
        raise ValueError("empty secret ID")

    import boto3

    endpoint_url = os.getenv("AWS_ENDPOINT_URL")
    if endpoint_url:
        client = boto3.client("secretsmanager", endpoint_url=endpoint_url)
    else:
        client = boto3.client("secretsmanager")

    response = client.get_secret_value(SecretId=secret_id)
    secret_string = response.get("SecretString")
    if not secret_string:
        raise ValueError(f"secret {secret_id!r} has no SecretString")

    try:
        parsed = json.loads(secret_string)
    except json.JSONDecodeError as exc:
        raise ValueError(f"parse secret JSON: {exc}") from exc

    if not isinstance(parsed, dict):
        raise ValueError("secret JSON must be an object")

    normalized: dict[str, str] = {}
    for key, value in parsed.items():
        if not isinstance(key, str):
            continue
        if value is None:
            continue
        normalized[key] = value if isinstance(value, str) else str(value)

    apply_secret_map(normalized)


def load_from_env_if_configured() -> None:
    secret_id = os.getenv(SECRET_ID_ENV, "").strip()
    if not secret_id:
        return
    load_from_secrets_manager(secret_id)


def must_load_from_env_if_configured() -> None:
    secret_id = os.getenv(SECRET_ID_ENV, "").strip()
    if not secret_id:
        return
    try:
        load_from_secrets_manager(secret_id)
    except Exception as exc:
        print(
            f"failed to load secrets from AWS Secrets Manager: {exc} "
            f"(env {SECRET_ID_ENV})",
            file=sys.stderr,
        )
        sys.exit(1)
