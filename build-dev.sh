#!/usr/bin/env bash

TAG="dev-v1.0.2"

if [ "$1" == "push" ]; then
    echo "pushing"
    docker buildx build -f Dockerfile.dev --push --platform=linux/amd64,linux/arm64 --progress=plain -t kuznetcovay/cdddru:${TAG} .
else
    echo "loading"
    docker buildx build -f Dockerfile.dev --load --platform=linux/amd64 --progress=plain -t kuznetcovay/cdddru:${TAG} .
fi
