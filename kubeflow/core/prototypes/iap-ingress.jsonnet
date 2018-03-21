// @apiVersion 0.1
// @name io.ksonnet.pkg.iap-ingress
// @description Provides ingress prototypes for setting up IAP on GKE.
// @shortDescription Ingress for IAP on GKE.
// @param name string Name for the component
// @optionalParam namespace string null Namespace to use for the components. It is automatically inherited from the environment if not set.
// @optionalParam secretName string envoy-ingress-tls The name of the secret containing the SSL certificates.
// @param ipName string The name of the global ip address to use.
// @optionalParam hostname string null The hostname associated with this ingress. Eg: mykubeflow.example.com
// @optionalParam issuer string letsencrypt-prod The cert-manager issuer name.

local k = import "k.libsonnet";
local iap = import "kubeflow/core/iap.libsonnet";

// updatedParams uses the environment namespace if
// the namespace parameter is not explicitly set
local updatedParams = params {
  namespace: if params.namespace == "null" then env.namespace else params.namespace
};

local name = import "param://name";
local namespace = updatedParams.namespace;
local secretName = import "param://secretName";
local ipName = import "param://ipName";
local hostname = import "param://hostname";
local issuer = import "param://issuer";

iap.parts(namespace).ingressParts(secretName, ipName, hostname, issuer)
