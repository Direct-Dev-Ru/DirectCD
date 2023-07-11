
# creating secrets in k3s cluster

kubectl create secret generic dockerhub-token -n test-app --from-literal=username=[your dockerhub username] \ --from-literal=token=[your secret token]} --from-literal=server=<https://index.docker.io/v1/>

kubectl create secret generic dockerhub-cred --from-file=[path-to-docker-config.json] -n test-app
