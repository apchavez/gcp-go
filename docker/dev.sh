#!/usr/bin/env bash
# Runs build/test/lint/terraform inside the same images used by CI, so nothing
# beyond Docker needs to be installed locally.
# Usage: ./docker/dev.sh <test|build|lint|terraform|shell>
set -euo pipefail
export MSYS_NO_PATHCONV=1
cd "$(dirname "$0")/.."
ROOT="$(pwd -W 2>/dev/null || pwd)"

IMAGE="golang:1.25"
TF_IMAGE="hashicorp/terraform:1.15"
CMD="${1:-test}"
DOCKER_TTY=""
[[ "$CMD" == "shell" ]] && DOCKER_TTY="-it"

run() {
  docker run --rm ${DOCKER_TTY:-} \
    -v "$ROOT":/workspace -w /workspace \
    -v gcp-go-mod-cache:/go/pkg/mod \
    "$IMAGE" bash -c "$1"
}

run_tf() {
  docker run --rm --entrypoint sh \
    -v "$ROOT/terraform":/workspace -w /workspace \
    "$TF_IMAGE" -c "$1"
}

case "$CMD" in
  test)      run "go test ./... -race" ;;
  build)     run "go build ./..." ;;
  lint)      run "go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest && golangci-lint run ./..." ;;
  terraform) run_tf "terraform fmt -check -diff && terraform init -backend=false && terraform validate" ;;
  shell)     run "bash" ;;
  *) echo "Usage: $0 {test|build|lint|terraform|shell}" >&2; exit 1 ;;
esac
