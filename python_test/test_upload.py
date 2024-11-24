import os
import sys
import requests
from dotenv import load_dotenv


def main(file_path):
    # Load environment variables from .env file
    load_dotenv()

    # Get authentication token from environment
    auth_token = os.getenv("AUTH_TOKEN")
    if not auth_token:
        print("ERROR: AUTH_TOKEN not found in .env file")
        sys.exit(1)

    # Setup base configuration
    base_url = "http://127.0.0.1:8081"
    headers = {"Authorization": f"Bearer {auth_token}"}

    # Step 1: Get presigned URL
    presign_response = requests.get(f"{base_url}/upload/presign", headers=headers)

    if presign_response.status_code != 200:
        print(f"Failed to get presigned URL: {presign_response.text}")
        sys.exit(1)

    presign_data = presign_response.json()["data"]
    upload_url = presign_data["upload_url"]
    upload_id = presign_data["file_id"]

    # Step 2: Upload file to S3
    try:
        with open(file_path, "rb") as file:
            s3_response = requests.put(upload_url, data=file)

        if s3_response.status_code != 200:
            print(f"Failed to upload file to S3: {s3_response.text}")
            sys.exit(1)
    except FileNotFoundError:
        print(f"Error: File {file_path} not found")
        sys.exit(1)

    # Step 3: Finalize upload
    finish_response = requests.get(
        f"{base_url}/upload/{upload_id}/complete", headers=headers
    )
    print(finish_response.status_code)
    if finish_response.status_code != 200:
        print(f"Failed to finalize upload: {finish_response.text}")
        sys.exit(1)

    data = finish_response.json()
    file_id = data["data"]["id"]
    print(data)
    print("Upload completed successfully!")
    resp = requests.get(f"{base_url}/file/{file_id}/dl", headers=headers)
    data = resp.json()
    print(data["data"])


if __name__ == "__main__":
    if len(sys.argv) != 2:
        print("Usage: python script.py <file_path>")
        sys.exit(1)

    main(sys.argv[1])
