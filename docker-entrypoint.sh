#!/bin/sh

sleep 5; 

mkdir -p /root/.ssh;

cp /.bind/configs/gitcred/id_rsa /root/.ssh/id_rsa;  

mkdir -p /root/.docker;
cp /.bind/configs/dockerconfig/config.json /root/.docker/;

mkdir -p /root/.kube;
cp /.bind/configs/kubeconfig/config /root/.kube/config;

chmod 400 /root/.ssh/id_rsa /root/.kube/config /root/.docker/config.json;

docker context create builder-context
docker buildx create builder-context --use

if [ $# -eq 0 ]; then
    tail -f /dev/null
else
    exec "$@"
fi
