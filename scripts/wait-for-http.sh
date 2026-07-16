#!/usr/bin/env sh
set -eu

url=${1:?usage: wait-for-http.sh URL [attempts] [delay_seconds]}
attempts=${2:-60}
delay=${3:-2}
i=0

until curl --fail --silent --show-error --max-time 3 "$url" >/dev/null; do
  i=$((i + 1))
  if [ "$i" -ge "$attempts" ]; then
    echo "timed out waiting for $url" >&2
    exit 1
  fi
  sleep "$delay"
done

echo "$url is ready"
