#!/bin/bash

# Script bricht bei Fehlern nicht mehr ab

# Fehlerbehandlung: Script bricht bei Fehler ab, aber Terminal bleibt offen
set +e
function check_error() {
  if [ $1 -ne 0 ]; then
    echo "[FEHLER] $2"
    exit 1
  fi
}


NAMESPACE="env-injector"

# Namespace anlegen, falls nicht vorhanden
kubectl get namespace "$NAMESPACE" >/dev/null 2>&1 || kubectl create namespace "$NAMESPACE"
kubectl get namespace "$NAMESPACE" >/dev/null 2>&1
if [ $? -ne 0 ]; then
  kubectl create namespace "$NAMESPACE"
  check_error $? "Namespace $NAMESPACE konnte nicht erstellt werden."
else
  echo "Namespace $NAMESPACE existiert bereits."
fi

# 1. Installiere cert-manager (Hinweis)
echo "Bitte stelle sicher, dass cert-manager im Cluster installiert ist!"


# 2. Erzeuge Issuer und Certificate
kubectl apply -f deployment/issuer.yaml
kubectl apply -f deployment/issuer.yaml
check_error $? "Issuer konnte nicht erstellt werden."
kubectl apply -f deployment/certificate.yaml
check_error $? "Certificate konnte nicht erstellt werden."

echo "Warte auf Secret (env-injector-webhook-tls) ..."
kubectl wait --for=condition=Ready certificate/env-injector-webhook-cert -n "$NAMESPACE" --timeout=60s
check_error $? "Certificate ist nicht bereit."

# 3. Patche das CA-Bundle in die MutatingWebhookConfiguration
cat deployment/mutatingwebhook.yaml | \
  deployment/webhook-patch-ca-bundle.sh > \
  deployment/mutatingwebhook-ca-bundle.yaml
check_error $? "CA-Bundle konnte nicht gepatcht werden."

echo "Patched CA-Bundle in mutatingwebhook-ca-bundle.yaml."

# 4. Deploy Ressourcen
kubectl apply -f deployment/configmap.yaml -n "$NAMESPACE"
kubectl apply -f deployment/deployment.yaml -n "$NAMESPACE"
kubectl apply -f deployment/service.yaml -n "$NAMESPACE"
kubectl apply -f deployment/mutatingwebhook-ca-bundle.yaml -n "$NAMESPACE"

kubectl apply -f deployment/configmap.yaml -n "$NAMESPACE"
check_error $? "ConfigMap konnte nicht erstellt werden."
kubectl apply -f deployment/deployment.yaml -n "$NAMESPACE"
check_error $? "Deployment konnte nicht erstellt werden."
kubectl apply -f deployment/service.yaml -n "$NAMESPACE"
check_error $? "Service konnte nicht erstellt werden."
kubectl apply -f deployment/mutatingwebhook-ca-bundle.yaml -n "$NAMESPACE"
check_error $? "MutatingWebhookConfiguration konnte nicht erstellt werden."

echo "Deployment abgeschlossen."
