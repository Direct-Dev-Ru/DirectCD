#!/usr/bin/env bash


# export NFS_SERVER=...

envsubst $NFS_SERVER < test.yaml | kubectl apply -f -

envsubst $NFS_SERVER < temp/pvc.yaml | kubectl apply -f -

envsubst $NFS_SERVER < k8s-deploy-cddru-new.yaml | kubectl apply -f -

