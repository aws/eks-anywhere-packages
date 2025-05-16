import json
import yaml
import subprocess
import os
import boto3
import base64
import argparse


def authenticate_ecr(account_id, region="us-west-2"):
    try:
        # Get ECR authentication token and perform docker login
        ecr_client = boto3.client("ecr", region_name=region)
        token = ecr_client.get_authorization_token()
        auth_token = token["authorizationData"][0]["authorizationToken"]
        creds = base64.b64decode(auth_token).decode("utf-8")
        username = creds.split(":")[0]
        password = creds.split(":")[1]
        registry_url = f"{account_id}.dkr.ecr.{region}.amazonaws.com"

        # Execute docker login
        login_cmd = [
            "docker",
            "login",
            "--username",
            username,
            "--password-stdin",
            registry_url,
        ]

        process = subprocess.Popen(
            login_cmd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
        )

        stdout, stderr = process.communicate(input=password.encode())

        if process.returncode != 0:
            raise Exception(f"Docker login failed: {stderr.decode()}")

        print(f"Successfully authenticated with ECR registry: {registry_url}")
        return registry_url

    except Exception as e:
        print(f"Authentication failed: {str(e)}")
        raise


def get_image_scan_findings(repository_name, image_id):
    try:
        ecr_client = boto3.client("ecr")

        response = ecr_client.describe_image_scan_findings(
            repositoryName=repository_name, imageId=image_id
        )

        # Extract scan findings
        scan_findings = response.get("imageScanFindings", {})

        # Print vulnerability counts by severity
        severity_counts = scan_findings.get("findingSeverityCounts", {})
        vulnerabilities = []
        for finding in scan_findings.get("enhancedFindings", []):
            if finding.get("severity") != "CRITICAL":
                continue
            vulnerability = {
                "severity": finding.get("severity"),
                "vulnerabilityId": finding.get("packageVulnerabilityDetails").get(
                    "vulnerabilityId"
                ),
                "sourceUrl": finding.get("packageVulnerabilityDetails").get(
                    "sourceUrl"
                ),
                "title": finding.get("title"),
                "type": finding.get("type"),
            }
            vulnerabilities.append(vulnerability)

        result = {
            "findingSeverityCounts": severity_counts,
            "vulnerabilities": vulnerabilities,
        }

        return result

    except ecr_client.exceptions.ImageNotFoundException:
        print("Image not found")
    except ecr_client.exceptions.RepositoryNotFoundException:
        print("Repository not found")
    except Exception as e:
        print(f"Error: {str(e)}")


def get_manifest_info(registry, repository, image_digest):
    image_uri = f"{registry}/{repository}@{image_digest}"
    try:
        # Run docker manifest inspect command
        result = subprocess.run(
            ["docker", "manifest", "inspect", image_uri],
            capture_output=True,
            text=True,
            check=True,
        )

        # Parse the JSON output
        manifest_data = json.loads(result.stdout)

        # Extract platform and digest information
        if "manifests" in manifest_data:
            # Handle multi-arch images
            platform_info = {}
            for manifest in manifest_data["manifests"]:
                platform = manifest.get("platform", {})
                arch = platform.get("architecture", "unknown")
                digest = manifest.get("digest", "")
                platform_info[arch] = {
                    "image_digest": digest,
                    "vulnerabilities": get_image_scan_findings(
                        repository, {"imageDigest": digest}
                    ),
                }
            return platform_info

    except subprocess.CalledProcessError as e:
        print(f"Error inspecting {image_uri}: {e.stderr}")
        return None
    except json.JSONDecodeError as e:
        print(f"Error parsing manifest for {image_uri}: {e}")
        return None


def process_bundle_yaml(file_path, registry_url):
    try:
        # Read the bundle.yaml file
        with open(file_path, "r") as file:
            bundle_data = yaml.safe_load(file)

        # Process each image in the bundle
        package_images = {}
        spec = bundle_data.get("spec")
        # Iterate through packages
        for package in spec.get("packages", []):
            package_name = package.get("name")
            if not package_name:
                continue

            package_images[package_name] = []

            # Get versions from source
            versions = package.get("source", {}).get("versions", [])
            for version in versions:
                # Get images list from each version
                images = version.get("images", [])
                for image in images:
                    manifest_info = get_manifest_info(
                        registry_url, image.get("repository"), image.get("digest")
                    )
                    image_info = {
                        "repository": image.get("repository"),
                        "digest": image.get("digest"),
                        "manifest_info": manifest_info,
                    }
                    package_images[package_name].append(image_info)
        return package_images
    except yaml.YAMLError as e:
        print(f"Error parsing YAML file: {e}")
        return None
    except FileNotFoundError:
        print(f"Bundle file not found: {file_path}")
        return None


def main():
    # Set up argument parser
    parser = argparse.ArgumentParser(
        description="Process bundle YAML file and scan ECR images for vulnerabilities"
    )
    parser.add_argument(
        "--bundle", "-b", required=True, help="Path to the bundle.yaml file"
    )
    parser.add_argument(
        "--account",
        "-a",
        default="724423470321",
        help="AWS account ID (default: 724423470321)",
    )
    parser.add_argument(
        "--region", "-r", default="us-west-2", help="AWS region (default: us-west-2)"
    )
    parser.add_argument(
        "--output",
        "-o",
        default="vulnerabilities_findings_critical.json",
        help="Output JSON file name",
    )

    args = parser.parse_args()

    # Authenticate with ECR
    registry_url = authenticate_ecr(args.account, args.region)

    # Process the bundle file
    vulnerabilities_details = process_bundle_yaml(args.bundle, registry_url)

    # Write results to file
    with open(args.output, "w", encoding="utf-8") as f:
        json.dump(vulnerabilities_details, f, indent=2, ensure_ascii=False)

    print(f"Vulnerability findings written to {args.output}")


if __name__ == "__main__":
    main()
