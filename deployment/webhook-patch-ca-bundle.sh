#!/bin/bash

ROOT=$(cd $(dirname $0)/../../; pwd)

set -o errexit
set -o nounset
set -o pipefail


# Extrahiere das CA-Bundle aus dem cert-manager Secret
export CA_BUNDLE=$(kubectl get secret env-injector-webhook-tls -n env-injector -o jsonpath='{.data.ca\.crt}')

if command -v envsubst >/dev/null 2>&1; then
    envsubst
else
    sed -e "s|\${CA_BUNDLE}|${CA_BUNDLE}|g"
fi
