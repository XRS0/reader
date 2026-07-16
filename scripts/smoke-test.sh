#!/usr/bin/env sh
set -eu

api=${API_URL:-http://localhost:8080}
frontend=${FRONTEND_URL:-http://localhost:3000}

request() {
  url=$1
  expected=${2:-200}
  status=$(curl --silent --show-error --output /dev/null --write-out '%{http_code}' "$url")
  if [ "$status" != "$expected" ]; then
    echo "expected HTTP $expected from $url, got $status" >&2
    exit 1
  fi
  echo "ok: $url -> $status"
}

request "$api/health/live"
request "$api/health/ready"
request "$frontend/"
