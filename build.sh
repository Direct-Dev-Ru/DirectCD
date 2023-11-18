#!/usr/bin/env bash

TAG="v1.0.25"

if [ "$1" == "push" ]; then
    echo "pushing"
    # docker buildx build --push -f Dockerfile.test --platform linux/arm/v7,linux/arm64/v8,linux/amd64 --tag kuznetcovay/buildxtest:buildx-latest .
    # docker buildx build --push --platform linux/arm/v7,linux/arm64/v8,linux/amd64 --progress=plain -t kuznetcovay/cdddru:${TAG} .
    docker buildx build --push --platform linux/arm/v7,linux/arm64/v8,linux/amd64 -t kuznetcovay/cdddru:${TAG} .
else
    echo "loading"
    docker buildx build --load --platform linux/amd64 --progress=plain -t kuznetcovay/cdddru:${TAG} .
fi

# docker buildx imagetools inspect --raw kuznetcovay/cdddru:${TAG}