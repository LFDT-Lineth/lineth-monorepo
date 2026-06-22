# Quick Start Guide

## Create a Local Kubernetes Cluster using K3S
If you don't have a Kubernetes cluster, you can create a local one using K3S

Note the bellow command will create kubectl config file at `~/.kube/k3s-server`
```
make -f k3s.mk k3s-reload
```

```
export KUBECONFIG=~/.kube/k3s-server
```

## Install Maru + Besu helm charts
```
make -f helm.mk redeploy
```

### Verify deployment
```
kubectl get pods
kubectl logs maru-XXX -f
```
