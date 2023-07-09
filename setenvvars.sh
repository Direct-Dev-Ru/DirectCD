#!/usr/bin/env bash

DOCKER_TOKEN=$(kubectl get secret dockerhub-token -n test-app -o jsonpath='{.data.token}' | base64 --decode)
DOCKER_USER=$(kubectl get secret dockerhub-token -n test-app -o jsonpath='{.data.username}' | base64 --decode)
DOCKER_SERVER=$(kubectl get secret dockerhub-token -n test-app -o jsonpath='{.data.server}' | base64 --decode)

export DOCKER_TOKEN
export DOCKER_USER
export DOCKER_SERVER
