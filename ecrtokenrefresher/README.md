# ECR Token Refresh
This repository deals with the issue of AWS Elastic Container Registry (ECR) credentials expiring every 12 hours.
This repository contains code to build a go program that will get an ECR Token with configured credentials and using the
go client for k8s update all target namespaces with a docker secret.

## Running the program
This program is meant to be run from a container inside a cluster that is trigger by a Kubernetes CronJob to keep 
credentials up to date.

### Running Locally
Local currently doesn't support IRSA.

```
# set your AWS Enviroment Variables
export AWS_ACCESS_KEY_ID=
export AWS_SECRET_ACCESS_KEY=
export AWS_REGION=us-west2

# Build binary
make
# Run Binary
./bin/ecr-refresh
```

### Running as a Kubernetes CronJob
```
# Create your secret
# Below Assumes your AWS Credentials are configured as env variables
# Adjust namespace to target namespace
kubectl create secret generic ecr-creds -n $TARGET_NAMESPACE \
  --from-literal=ID=$(AWS_ACCESS_KEY_ID) \
  --from-literal=SECRET=$(AWS_SECRET_ACCESS_KEY) \
  --from-literal=REGION=$(AWS_REGION)
```
The below contains env variable definition to be added to a cronjob

```yaml
kind: CronJob
containers:
  - name: ...
    image: ...
    env:
        - name: ECR_TOKEN_SECRET_NAME
          value: ecr-token
        - name: AWS_REGION
          valueFrom:
            secretKeyRef:
              name: ecr-creds
              key: REGION
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: ecr-creds
              key: ID
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: ecr-creds
              key: SECRET

```

### IAM roles for service accounts (IRSA)
Assuming IRSA and the [webhook](https://github.com/aws/amazon-eks-pod-identity-webhook) is setup, the program expects environment variables
```AWS_ROLE_ARN``` and ```AWS_WEB_IDENTITY_TOKEN_FILE``` to be configured. Note these variables are normally injected
by the webhook if the annotations are added to the container spec.

Using the role arn and web identity token the program will assume the role with web identity and populate
the standard AWS Credentials as environment variables before getting the ECR Token. Make sure the role has policy with 
at minimum ecr:GetAuthorizationToken permissions set.