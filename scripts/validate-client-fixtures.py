#!/usr/bin/env python3
"""Ensure client GraphQL v1 fixtures stay byte-semantically aligned with BFF."""

from __future__ import annotations

import json
import sys
from pathlib import Path


FIXTURES = ("catalog.json", "cart.json", "order.json")
CLIENT_FIXTURE_DIRS = {
    "Next.js": Path("storemesh-frontend/tests/fixtures/bff/v1"),
    "Android": Path("storemesh-android/app/src/test/resources/bff/v1"),
    "iOS": Path("storemesh-ios/storemesh-iosTests/Fixtures/bff/v1"),
}
CANONICAL_FIXTURE_DIR = Path("storemesh-bff/api/graphql/fixtures/v1")


def load_fixtures(directory: Path, label: str) -> dict[str, object]:
    if not directory.is_dir():
        raise ValueError(f"{label} fixture directory not found: {directory}")

    actual = {path.name for path in directory.glob("*.json")}
    expected = set(FIXTURES)
    if actual != expected:
        raise ValueError(
            f"{label} fixture set mismatch at {directory}: "
            f"missing={sorted(expected - actual)}, extra={sorted(actual - expected)}"
        )

    loaded: dict[str, object] = {}
    for name in FIXTURES:
        path = directory / name
        try:
            loaded[name] = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError) as error:
            raise ValueError(f"invalid JSON in {path}: {error}") from error
    return loaded


def main() -> int:
    workspace = Path(sys.argv[1] if len(sys.argv) > 1 else ".").resolve()
    canonical = load_fixtures(workspace / CANONICAL_FIXTURE_DIR, "BFF canonical")
    failures: list[str] = []

    for client, relative_path in CLIENT_FIXTURE_DIRS.items():
        try:
            client_fixtures = load_fixtures(workspace / relative_path, client)
        except ValueError as error:
            failures.append(str(error))
            continue

        for fixture in FIXTURES:
            if client_fixtures[fixture] != canonical[fixture]:
                failures.append(f"{client} fixture differs from BFF canonical: {fixture}")

    if failures:
        print("BFF/client GraphQL fixture parity failed:", file=sys.stderr)
        for failure in failures:
            print(f"- {failure}", file=sys.stderr)
        return 1

    print("BFF, Next.js, Android, and iOS v1 fixtures match (catalog, cart, order).")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
