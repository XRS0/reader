#!/usr/bin/env sh
set -eu

env_file=${1:-.env}
if [ ! -f "$env_file" ]; then
  echo "$env_file does not exist; copy .env.example first" >&2
  exit 1
fi

if grep -Eq '(^|=)CHANGE_ME' "$env_file"; then
  echo "$env_file still contains CHANGE_ME values" >&2
  exit 1
fi

if grep -Eq '^(APP_ENV=production|COOKIE_SECURE=true)$' "$env_file"; then
  if grep -Eq '(^|=)(bookflow_dev_only|BookFlow-demo-only)' "$env_file"; then
    echo "development-only credentials are not allowed for production" >&2
    exit 1
  fi
fi

echo "$env_file passed the basic secret-placeholder check"
