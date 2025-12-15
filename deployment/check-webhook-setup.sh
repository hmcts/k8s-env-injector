#!/bin/bash
# Prüft Service, WebhookConfig und DNS-Namen im Zertifikat

set -e

# 1. Service-Name und Namespace prüfen
kubectl get svc env-injector-webhook-svc -n env-injector || { echo "Service env-injector-webhook-svc im Namespace env-injector NICHT gefunden!"; exit 1; }
echo "Service env-injector-webhook-svc im Namespace env-injector gefunden."

# 2. Webhook-Konfiguration prüfen
kubectl get mutatingwebhookconfiguration env-injector-webhook-cfg -o yaml | grep "service:" -A 5

# 3. DNS-Namen im Zertifikat anzeigen
kubectl get secret env-injector-webhook-tls -n env-injector -o jsonpath='{.data.tls\.crt}' | base64 -d | openssl x509 -noout -text | grep -A1 "DNS:"
