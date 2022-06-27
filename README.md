# Kubernetes Mutating Admission Webhook for environment injection

This repo hosts a [MutatingAdmissionWebhook](https://kubernetes.io/docs/admin/admission-controllers/#mutatingadmissionwebhook-beta-in-19) that injects environment variables, dns options and node affinity into pod containers prior to persistence of the object.
Node affinity is currently limited to RequiredDuringSchedulingIgnoredDuringExecution selector terms.

## Prerequisites

Kubernetes 1.22.0 or above with the `admissionregistration.k8s.io/v1` API enabled. Verify that by the following command:
```
kubectl api-versions | grep admissionregistration.k8s.io/v1
```
The result should be:
```
admissionregistration.k8s.io/v1
```

In addition, the `MutatingAdmissionWebhook` and `ValidatingAdmissionWebhook` admission controllers should be added and listed in the correct order in the admission-control flag of kube-apiserver.

## Build


1. Build and push docker image
   
```
./build
```

## Deploy

1. Create a signed cert/key pair and store it in a Kubernetes `secret` that will be consumed by env-injector deployment
```
./deployment/webhook-create-signed-cert.sh \
    --service env-injector-webhook-svc \
    --secret env-injector-webhook-certs \
    --namespace default
```

2. Patch the `MutatingWebhookConfiguration` by set `caBundle` with correct value from Kubernetes cluster
```
cat deployment/mutatingwebhook.yaml | \
    deployment/webhook-patch-ca-bundle.sh > \
    deployment/mutatingwebhook-ca-bundle.yaml
```

3. Deploy resources
```
kubectl create -f deployment/configmap.yaml
kubectl create -f deployment/deployment.yaml
kubectl create -f deployment/service.yaml
kubectl create -f deployment/mutatingwebhook-ca-bundle.yaml
```

## Verify

1. The environment inject webhook should be running
```
$ kubectl get pods
NAME                                                  READY     STATUS    RESTARTS   AGE
env-injector-webhook-deployment-bbb689d69-882dd   1/1       Running   0          5m
$ kubectl get deployment
NAME                                  DESIRED   CURRENT   UP-TO-DATE   AVAILABLE   AGE
env-injector-webhook-deployment   1         1         1            1           5m
```

2. Label the default namespace with `env-injector=enabled`
```
$ kubectl label namespace default hmcts.github.com/envInjector=enabled
$ kubectl get namespace -L hmcts.github.com/envInjector
NAME              STATUS   AGE    ENVINJECTOR
default           Active   4d3h   enabled
kube-node-lease   Active   4d3h   
kube-public       Active   4d3h   
kube-system       Active   4d3h   
```

3. Deploy an app in Kubernetes cluster, take `sleep` app as an example
```
$ cat <<EOF | kubectl create -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: sleep
spec:
  replicas: 1
  selector:
    matchLabels:
      app: nginx
  template:
    metadata:
      labels:
        app: sleep
    spec:
      containers:
      - name: sleep
        image: hmctspublic.azurecr.io/docker-curl
        command: ["sleep","1d"]
        imagePullPolicy: Always
EOF
```

4. Verify environment injected by describing the pod
```
$ kubectl describe pod ...
```

## Helm chart

A Helm chart is also available, see [env-injector-webhook](charts/env-injector-webhook/Chart.yaml).
This can be installed in a single step using helm 2 or 3, e.g.
```
$ helm upgrade env-injector-webhook env-injector-webhook --install --namespace admin
```

*Note*: As the pods and service need to have:
- a secret containing a signed certificate and key
- a mutating webhook patched with the CA Bundle
the script executed from `pre-install-job.yaml` takes care of creating them executing as a helm pre-install + pre-upgrade hook. 
This allows the installation/upgrade steps to execute in the right order, but has the (unfortunate) side effect of leaving 
around the secret and mutating webhook when the chart is deleted. 
For that reason a pre-upgrade + post-delete helm hook takes care of deleting secret and admission webhook.

## Notes

This repo is based on the excellent tutorial available at: [morvencao/kube-mutating-webhook-tutorial](https://github.com/morvencao/kube-mutating-webhook-tutorial) 
