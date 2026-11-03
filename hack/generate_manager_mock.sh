#!/usr/bin/env bash
set -euo pipefail

mockgen -destination=mocks/manager.go -package=mocks sigs.k8s.io/controller-runtime/pkg/manager Manager
# Reflection resolves client-go's public recorder alias to its internal definition.
sed -i 's@k8s.io/client-go/tools/internal/events@k8s.io/client-go/tools/events@' mocks/manager.go
gofmt -w mocks/manager.go
