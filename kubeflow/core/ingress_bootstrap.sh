#!/bin/bash

set -x
set -e

apk add --update openssl

# This is a workaround until this is resolved: https://github.com/kubernetes/ingress-gce/pull/388
# The long-term solution is to use a managed SSL certificate on GKE once the feature is GA.

# Install kubectl
K8S_VERSION=v1.11.0
curl -sfSL https://storage.googleapis.com/kubernetes-release/release/${K8S_VERSION}/bin/linux/amd64/kubectl > /usr/local/bin/kubectl
chmod +x /usr/local/bin/kubectl

# The ingress is initially created without a tls spec.
# Wait until cert-manager generates the certificate using the http-01 challenge on the GCLB ingress.
# After the certificate is obtained, patch the ingress with the tls spec to enable SSL on the GCLB.

# Wait for certificate.
(until kubectl -n ${NAMESPACE} get secret ${TLS_SECRET_NAME} 2>/dev/null; do echo "Waiting for certificate..." ; sleep 2; done)

kubectl -n kubeflow patch ingress ${INGRESS_NAME} --type='json' -p '[{"op": "add", "path": "/spec/tls", "value": [{"secretName": "envoy-ingress-tls"}]}]'

echo "Done"
