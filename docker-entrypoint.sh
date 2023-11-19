#!/bin/sh

sleep 30

mkdir -p /root/.ssh
cp /run/configs/gitcred/id_rsa /root/.ssh/id_rsa

mkdir -p /root/.docker
rm /root/.docker/config.json
cp /run/configs/dockerconfig/config.json /root/.docker/config.json
# export DOCKER_CONFIG=/root/.docker/;

mkdir -p /root/.kube
cp /run/configs/kubeconfig/config /root/.kube/config
# export KUBECONFIG=/root/.kube/config;

chmod 400 /root/.ssh/id_rsa
chmod 400 /root/.docker/config.json
chmod 400 /root/.kube/config

# apk add zsh-vcs

docker context create builder-context
docker buildx create --use --name mybuilder --node mybuilder \
  --driver-opt env.BUILDKIT_CPU_LIMIT=800m \
  --driver-opt env.BUILDKIT_MEMORY_LIMIT=800m

if [ $# -eq 0 ]; then
  echo "$@" && echo
  # tail -f /dev/null
else
  exec "$@"
fi
# MODE="development" go run . jobs-dev/config-dev.yaml
