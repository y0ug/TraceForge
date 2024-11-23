#!/bin/sh

set -e

POLICY=$(
  cat <<EOF
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Action": [
        "s3:GetBucketLocation",
        "s3:ListBucket"
      ],
      "Effect": "Allow",
      "Resource": "arn:aws:s3:::${MINIO_BUCKET}"
    },
    {
      "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:DeleteObject"
      ],
      "Effect": "Allow",
      "Resource": "arn:aws:s3:::${MINIO_BUCKET}/*"
    }
  ]
}
EOF
)

# Wait for MinIO to be ready
until (mc alias set myminio http://minio:9000 "${MINIO_ROOT_USER}" "${MINIO_ROOT_PASSWORD}"); do
  echo 'Waiting for MinIO...'
  sleep 5
done

# Create the bucket (ignore if it exists)
mc mb myminio/"${MINIO_BUCKET}" || true

# Create the user
mc admin user add myminio "${MINIO_USER}" "${MINIO_USER_PASSWORD}"

# Define the policy name
POLICY_NAME="${MINIO_BUCKET}-policy"

# Add and attach the custom policy
echo "$POLICY" | mc admin policy create myminio "${POLICY_NAME}" /dev/stdin
mc admin policy attach myminio "${POLICY_NAME}" --user="${MINIO_USER}"
