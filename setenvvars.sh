#!/usr/bin/env bash

DOCKER_PASSWORD=$(kubectl get secret dockerhub-token -n test-app -o jsonpath='{.data.token}' | base64 --decode)
DOCKER_USER=$(kubectl get secret dockerhub-token -n test-app -o jsonpath='{.data.username}' | base64 --decode)
DOCKER_SERVER=$(kubectl get secret dockerhub-token -n test-app -o jsonpath='{.data.server}' | base64 --decode)

export DOCKER_PASSWORD
export DOCKER_USER
export DOCKER_SERVER

DOCKER_BUILDKIT=1
export DOCKER_BUILDKIT

kubectl get secret kubeconfig -n test-app -o jsonpath='{.data.config}' | base64 --decode | cat >| .kubeconfig