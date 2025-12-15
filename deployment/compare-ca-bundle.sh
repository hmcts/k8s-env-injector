#!/bin/bash
# Vergleicht das CA-Bundle im Secret mit dem in der MutatingWebhookConfiguration

SECRET_CA=$(kubectl get secret env-injector-webhook-tls -n env-injector -o jsonpath='{.data.ca\.crt}')
WEBHOOK_CA=$(kubectl get mutatingwebhookconfiguration env-injector-webhook-cfg -o jsonpath='{.webhooks[0].clientConfig.caBundle}')

if [ "$SECRET_CA" = "$WEBHOOK_CA" ]; then
  echo "CA-Bundle in Secret und MutatingWebhookConfiguration sind IDENTISCH."
else
  echo "CA-Bundle in Secret und MutatingWebhookConfiguration sind UNTERSCHIEDLICH!"
fi
