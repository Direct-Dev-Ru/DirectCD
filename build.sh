#!/usr/bin/env bash

TAG="v1.0.15"

if [ "$1" == "push" ]; then
    echo "pushing"
    docker buildx build --push --platform=linux/amd64,linux/arm64 --progress=plain -t kuznetcovay/cdddru:${TAG} .
else
    echo "loading"
    docker buildx build --load --platform=linux/amd64 --progress=plain -t kuznetcovay/cdddru:${TAG} .
fi

# docker buildx imagetools inspect --raw kuznetcovay/cdddru:${TAG}