# Build the manager binary
FROM golang:1.18 as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN GOPROXY=direct go mod download

# Copy the go source
COPY cmd/ cmd/
COPY pkg/ pkg/

# Build
ENV GOTRACEBACK=single
ARG SKAFFOLD_GO_GCFLAGS
RUN GOPROXY=direct CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -gcflags="${SKAFFOLD_GO_GCFLAGS}" -a -o ecr-token-refresh cmd/ecr-token-refresher/main.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/ecr-token-refresh .
ENV GOTRACEBACK=single
USER 65532

ENTRYPOINT ["/ecr-token-refresh"]