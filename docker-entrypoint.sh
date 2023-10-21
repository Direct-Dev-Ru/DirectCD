#!/bin/sh

sleep 3; 

mkdir -p /root/.ssh;

# cp /.bind/configs/gitcred/id_rsa /root/.ssh/id_rsa;  

mkdir -p /root/.docker;
rm /root/.docker/config.json;
# cp /.bind/configs/dockerconfig/config.json /root/.docker/config.json;

mkdir -p /root/.kube;
# cp /.bind/configs/kubeconfig/config /root/.kube/config;

# chmod 400 /root/.ssh/id_rsa 
# chmod 400 /root/.docker/config.json;
# chmod 400 /root/.kube/config /root/.docker/config.json;

# apk add zsh-vcs

docker context create builder-context
docker buildx create builder-context --use

if [ $# -eq 0 ]; then
    tail -f /dev/null
else
    exec "$@"
fi
# MODE="development" go run . jobs-dev/config-dev.yaml 