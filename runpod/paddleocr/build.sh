#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

IMAGE="${IMAGE:-themylogin/ownocr-paddleocr-runpod:latest}"
push=0
args=()

usage() {
  cat <<TEXT
usage: [IMAGE=<tag>] ./build.sh [--push] [docker build options]

  IMAGE     image tag to build. Default: $IMAGE
  --push    push the tag after a successful build
  --help    print this text

RunPod needs a tag in a registry it can pull, e.g. docker.io/<user>/...:latest.
TEXT
}

for arg in "$@"; do
  case "$arg" in
    --push) push=1 ;;
    --help|-h) usage; exit 0 ;;
    *) args+=("$arg") ;;
  esac
done

if [ ! -f hf_token ]; then
  echo "error: hf_token is missing. Put a Hugging Face read token in it." >&2
  exit 1
fi

DOCKER_BUILDKIT=1 docker build --secret id=hf_token,src=./hf_token \
  -t "$IMAGE" ${args[@]+"${args[@]}"} .

if [ "$push" = "1" ]; then
  docker push "$IMAGE"
fi
