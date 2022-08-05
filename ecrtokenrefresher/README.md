# ECR Token Refresh
This repository deals with the issue of AWS Elastic Container Registry (ECR) credentials expiring every 12 hours.
This repository contains code to build a go program that will get an ECR Token with configured credentials and using the
go client for k8s update all target namespaces with a docker secret.

## Running the program
This program is meant to be run from a container inside a cluster that is trigger by a Kubernetes CronJob to keep 
credentials up to date.

### Running Locally

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
  - secretname: ...
    image: ...
    env:
        - secretname: ECR_TOKEN_SECRET_NAME
          value: ecr-token
        - secretname: AWS_REGION
          valueFrom:
            secretKeyRef:
              secretname: ecr-creds
              key: REGION
        - secretname: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              secretname: ecr-creds
              key: ID
        - secretname: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              secretname: ecr-creds
              key: SECRET
```