#!/usr/bin/env bash

export DOCKER_TOKEN=$(kubectl get secret dockerhub-token -n test-app -o jsonpath='{.data.token}' | base64 --decode)
export DOCKER_USER=$(kubectl get secret dockerhub-token -n test-app -o jsonpath='{.data.username}' | base64 --decode)
export DOCKER_SERVER=$(kubectl get secret dockerhub-token -n test-app -o jsonpath='{.data.server}' | base64 --decode)