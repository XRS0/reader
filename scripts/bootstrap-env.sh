#!/usr/bin/env sh
set -eu

target=${1:-.env}
if [ -e "$target" ] && [ "${FORCE:-0}" != "1" ]; then
  echo "$target already exists; refusing to overwrite (set FORCE=1 explicitly)" >&2
  exit 1
fi
command -v openssl >/dev/null 2>&1 || { echo "openssl is required" >&2; exit 1; }

cp .env.example "$target"

replace() {
  key=$1
  value=$2
  tmp="${target}.tmp.$$"
  awk -v key="$key" -v value="$value" 'BEGIN { FS="=" } $1 == key { $0=key "=" value } { print }' "$target" >"$tmp"
  mv "$tmp" "$target"
}

secret() { openssl rand -hex "${1:-32}"; }

replace JWT_SIGNING_KEY "$(secret 48)"
replace CSRF_SECRET "$(secret 48)"
replace UPTRACE_SECRET "$(secret 48)"
replace UPTRACE_ADMIN_PASSWORD "$(secret 24)"
replace UPTRACE_USER_TOKEN "$(secret 32)"
replace UPTRACE_PROJECT_TOKEN "$(secret 32)"
replace UPTRACE_POSTGRES_PASSWORD "$(secret 24)"
replace UPTRACE_CLICKHOUSE_PASSWORD "$(secret 24)"
replace UPTRACE_REDIS_PASSWORD "$(secret 24)"

chmod 600 "$target" 2>/dev/null || true
echo "created $target with generated local secrets"
echo "review development-only PostgreSQL, RustFS and seed credentials before starting"
