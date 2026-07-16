#!/usr/bin/env python3
"""Verify that the OpenAPI operation inventory matches the Gin router inventory.

The checker intentionally uses only the Python standard library. It does not try to
infer request/response schemas; Redocly validates those. It prevents a route from
being added to Gin without also being added to the public contract (and vice versa).
"""

from __future__ import annotations

import argparse
import re
import sys
from collections import Counter
from pathlib import Path

HTTP_METHODS = {"GET", "POST", "PUT", "PATCH", "DELETE"}
CALL_RE = re.compile(
    r'(?m)^\s*(?P<receiver>[A-Za-z_]\w*)\.(?P<method>GET|POST|PUT|PATCH|DELETE)\('
    r'\s*"(?P<path>/[^"\\]*)"'
)
GROUP_RE = re.compile(
    r'(?m)^\s*(?P<child>[A-Za-z_]\w*)\s*:=\s*(?P<parent>[A-Za-z_]\w*)\.Group\('
    r'\s*"(?P<path>/[^"\\]*|)"\s*\)'
)
OPENAPI_PATH_RE = re.compile(r"^  (?P<path>/[^:]+):\s*$")
OPENAPI_METHOD_RE = re.compile(r"^    (?P<method>get|post|put|patch|delete):\s*$")
GIN_PARAM_RE = re.compile(r":([A-Za-z_]\w*)")


def join(prefix: str, path: str) -> str:
    value = f"{prefix.rstrip('/')}/{path.lstrip('/')}"
    return value if value.startswith("/") else f"/{value}"


def normalize_gin_path(path: str) -> str:
    return GIN_PARAM_RE.sub(r"{\1}", path)


def gin_operations(http_dir: Path) -> list[tuple[str, str]]:
    server = http_dir / "server.go"
    source = server.read_text(encoding="utf-8")
    prefixes: dict[str, str] = {"r": ""}

    # Group declarations are ordered in server.go, but iterate as a guard if the
    # source is rearranged so a child appears before its parent.
    groups = list(GROUP_RE.finditer(source))
    pending = groups
    while pending:
        next_pending = []
        changed = False
        for match in pending:
            parent = match.group("parent")
            if parent not in prefixes:
                next_pending.append(match)
                continue
            prefixes[match.group("child")] = join(prefixes[parent], match.group("path"))
            changed = True
        if not changed:
            unresolved = ", ".join(m.group("child") for m in next_pending)
            raise ValueError(f"cannot resolve Gin group prefix(es): {unresolved}")
        pending = next_pending

    operations: list[tuple[str, str]] = []
    for match in CALL_RE.finditer(source):
        receiver = match.group("receiver")
        if receiver not in prefixes:
            # Route declarations in register*Routes functions are collected from
            # their own files below; this only skips unrelated method calls.
            continue
        path = normalize_gin_path(join(prefixes[receiver], match.group("path")))
        operations.append((match.group("method"), path))

    # Every register*Routes function receives the protected /api/v1 group as `r`.
    for go_file in sorted(http_dir.glob("*.go")):
        if go_file.name == "server.go" or go_file.name.endswith("_test.go"):
            continue
        text = go_file.read_text(encoding="utf-8")
        for match in CALL_RE.finditer(text):
            if match.group("receiver") != "r":
                continue
            path = normalize_gin_path(join("/api/v1", match.group("path")))
            operations.append((match.group("method"), path))
    return operations


def openapi_operations(spec: Path) -> list[tuple[str, str]]:
    operations: list[tuple[str, str]] = []
    current_path: str | None = None
    in_paths = False
    for line in spec.read_text(encoding="utf-8").splitlines():
        if line == "paths:":
            in_paths = True
            continue
        if in_paths and line and not line.startswith(" "):
            break
        if not in_paths:
            continue
        path_match = OPENAPI_PATH_RE.match(line)
        if path_match:
            current_path = path_match.group("path")
            continue
        method_match = OPENAPI_METHOD_RE.match(line)
        if method_match and current_path:
            operations.append((method_match.group("method").upper(), current_path))
    return operations


def duplicates(items: list[tuple[str, str]]) -> set[tuple[str, str]]:
    return {item for item, count in Counter(items).items() if count > 1}


def render(items: set[tuple[str, str]]) -> str:
    return "\n".join(f"  {method:6} {path}" for method, path in sorted(items, key=lambda x: (x[1], x[0])))


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--spec", type=Path, default=Path("backend/openapi/openapi.yaml"))
    parser.add_argument("--http-dir", type=Path, default=Path("backend/internal/httpapi"))
    args = parser.parse_args()

    actual_list = gin_operations(args.http_dir)
    contract_list = openapi_operations(args.spec)
    actual, contract = set(actual_list), set(contract_list)
    failed = False

    for label, values in (("Gin", duplicates(actual_list)), ("OpenAPI", duplicates(contract_list))):
        if values:
            failed = True
            print(f"Duplicate {label} operations:\n{render(values)}", file=sys.stderr)
    missing = actual - contract
    extra = contract - actual
    if missing:
        failed = True
        print(f"Missing from OpenAPI:\n{render(missing)}", file=sys.stderr)
    if extra:
        failed = True
        print(f"Not implemented by Gin:\n{render(extra)}", file=sys.stderr)
    if failed:
        return 1

    print(
        f"OpenAPI route inventory matches Gin: {len(actual)} operations across "
        f"{len({path for _, path in actual})} paths."
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
