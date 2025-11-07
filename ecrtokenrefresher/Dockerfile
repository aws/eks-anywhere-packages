# Build the manager binary
FROM golang:1.25 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY pkg/ pkg/
COPY cmd/ cmd/

# Build
RUN GOPROXY=direct CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -o ecr-token-refresh ./cmd/ecr-token-refresher

FROM gcr.io/distroless/static:debug-nonroot
WORKDIR /
COPY --from=builder /workspace/ecr-token-refresh .
# https://github.com/GoogleContainerTools/distroless/blob/e3c79deb1c576afd2e7e77b96996e2e1abdc6937/base/base.bzl#L8
USER 65532:65532

ENTRYPOINT ["/ecr-token-refresh"]
