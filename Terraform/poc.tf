resource "scaleway_iam_application" "sbapp" {
  name = "sbapp"
}

# Create an API key with read/write permissions
resource "scaleway_iam_api_key" "sbapp_access_token" {
  description    = "API key for sbapp"
  application_id = scaleway_iam_application.sbapp.id
}


# Create bucket to save email
resource "scaleway_object_bucket" "sbapp" {
  name = "sbapp-poc"
}


# Setup bucket policy 
resource "scaleway_iam_policy" "sbapp_policy" {
  name           = "sbapp_policy"
  application_id = scaleway_iam_application.sbapp.id
  rule {
    project_ids          = [var.scw_project_id]
    permission_set_names = ["ObjectStorageObjectsRead", "ObjectStorageObjectsWrite", "ObjectStorageObjectsDelete"]
  }
}

resource "scaleway_object_bucket_policy" "sbapp_bucket_policy" {
  bucket = scaleway_object_bucket.sbapp.id
  policy = jsonencode(
    {
      Id      = "policy"
      Version = "2023-04-17",
      Statement = [
        {
          Sid    = "Allow admin access",
          Effect = "Allow",
          Principal = {
            SCW = "user_id:bfccf1f7-b546-4c59-b9da-6b60337f3084"
          },
          Action = [
            "*",
          ]
          Resource = [
            "${scaleway_object_bucket.sbapp.name}",
            "${scaleway_object_bucket.sbapp.name}/*"
          ]
        },
        {
          Sid    = "Allow application volsync read/write access",
          Effect = "Allow",
          Principal = {
            SCW = "application_id:${scaleway_iam_application.sbapp.id}"
          },
          Action = [
            "s3:GetObject",
            "s3:PutObject",
            "s3:DeleteObject",
            "s3:ListBucket"
          ]
          Resource = [
            "${scaleway_object_bucket.sbapp.name}",
            "${scaleway_object_bucket.sbapp.name}/*"
          ]
        }
      ]
    }
  )
}


# Activate SQS for the project
# terraform import scaleway_mnq_sqs.main  fr-par/802b6dc7-d07d-45cc-be79-8822053fdf71
resource "scaleway_mnq_sqs" "main" {
}

resource "scaleway_mnq_sqs_credentials" "sbapp" {
  project_id = scaleway_mnq_sqs.main.project_id
  name       = "sbapp-credentials"

  permissions {
    can_manage  = true
    can_receive = true
    can_publish = true
  }
}

resource "scaleway_mnq_sqs_queue" "sbapp" {
  project_id   = scaleway_mnq_sqs.main.project_id
  name         = "sbapp"
  sqs_endpoint = scaleway_mnq_sqs.main.endpoint
  access_key   = scaleway_mnq_sqs_credentials.sbapp.access_key
  secret_key   = scaleway_mnq_sqs_credentials.sbapp.secret_key
}

locals {
  env_worker = <<-EOT
    S3_BUCKET_NAME = "${scaleway_object_bucket.sbapp.name}"
    S3_ACCESS_KEY = "${scaleway_iam_api_key.sbapp_access_token.access_key}"
    S3_SECRET_KEY = "${scaleway_iam_api_key.sbapp_access_token.secret_key}"
    S3_ENDPOINT = "${scaleway_object_bucket.sbapp.api_endpoint}"
    S3_REGION = "${scaleway_object_bucket.sbapp.region}"
    SQS_ACCESS_KEY = "${scaleway_mnq_sqs_credentials.sbapp.access_key}"
    SQS_SECRET_KEY = "${scaleway_mnq_sqs_credentials.sbapp.secret_key}"
    SQS_ENDPOINT = "${scaleway_mnq_sqs_queue.sbapp.sqs_endpoint}"
    SQS_QUEUE_URL = "${scaleway_mnq_sqs_queue.sbapp.url}"
  EOT
}
# SQS_QUEUE_URL = "https://sqs.mnq.fr-par.scaleway.com/project-802b6dc7-d07d-45cc-be79-8822053fdf71/email-worker"
# SQS_ENDPOINT = "https://sqs.mnq.fr-par.scaleway.com"

resource "local_file" "sbapp" {
  filename = "${path.module}/output/sbapp.env"
  content  = local.env_worker
}
