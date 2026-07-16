#!/usr/bin/env sh
set -eu

endpoint=${RUSTFS_ENDPOINT:-http://rustfs:9000}
region=${AWS_DEFAULT_REGION:-us-east-1}
buckets=${RUSTFS_BUCKETS:-"books-original books-content books-assets books-covers user-exports"}

echo "waiting for RustFS at ${endpoint}"
i=0
until aws --endpoint-url "$endpoint" s3api list-buckets >/dev/null 2>&1; do
  i=$((i + 1))
  if [ "$i" -ge 60 ]; then
    echo "RustFS did not become ready after 120 seconds" >&2
    exit 1
  fi
  sleep 2
done

for bucket in $buckets; do
  if aws --endpoint-url "$endpoint" s3api head-bucket --bucket "$bucket" >/dev/null 2>&1; then
    echo "bucket ${bucket} already exists"
    continue
  fi

  # us-east-1 rejects an explicit LocationConstraint in S3-compatible APIs.
  if [ "$region" = "us-east-1" ]; then
    aws --endpoint-url "$endpoint" s3api create-bucket --bucket "$bucket" >/dev/null
  else
    aws --endpoint-url "$endpoint" s3api create-bucket --bucket "$bucket" \
      --create-bucket-configuration "LocationConstraint=${region}" >/dev/null
  fi
  echo "created bucket ${bucket}"
done

echo "RustFS buckets are ready"
