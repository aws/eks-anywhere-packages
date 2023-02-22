FROM golang:1.18-buster
ENV GOTRACEBACK=single
ENV GOPROXY=direct
WORKDIR /app
COPY go.mod .
COPY go.sum .
COPY cmd/ cmd/
COPY ecr-credential-provider /eksa-binaries/
COPY aws_signing_helper /eksa-binaries/
COPY pkg/ pkg/
ARG SKAFFOLD_GO_GCFLAGS
RUN go build -gcflags="${SKAFFOLD_GO_GCFLAGS}" -o app cmd/aws-credential-provider/*.go

CMD ["/app/app"]
