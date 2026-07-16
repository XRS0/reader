#!/usr/bin/env sh
set -eu

name=${1:-}
if [ -z "$name" ]; then
  echo "usage: $0 <migration_name>" >&2
  exit 64
fi

case "$name" in
  *[!a-zA-Z0-9_-]*)
    echo "migration name may contain only letters, digits, underscores and hyphens" >&2
    exit 64
    ;;
esac

root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
directory="$root/backend/migrations"
mkdir -p "$directory"

# UTC plus nanoseconds avoids collisions between developers creating migrations
# during the same second. BSD date does not expose nanoseconds, so use seconds
# and fail rather than overwrite an existing pair.
version=$(date -u +%Y%m%d%H%M%S)
up="$directory/${version}_${name}.up.sql"
down="$directory/${version}_${name}.down.sql"

if [ -e "$up" ] || [ -e "$down" ]; then
  echo "migration version collision; wait one second and retry" >&2
  exit 1
fi

printf '%s\n' 'BEGIN;' '' '-- Write the forward migration here.' '' 'COMMIT;' >"$up"
printf '%s\n' 'BEGIN;' '' '-- Write the rollback migration here.' '' 'COMMIT;' >"$down"
printf 'created %s\ncreated %s\n' "$up" "$down"
