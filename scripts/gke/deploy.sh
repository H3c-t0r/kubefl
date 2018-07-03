#!/usr/bin/env bash
# This script creates a kubeflow deployment on GCP
# * It checks for kubectl, gcloud, ks
# * Uses default PROJECT, ZONE, EMAIL from gcloud config
# *

set -xe

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
source "${SCRIPT_DIR}/../util.sh"
KUBEFLOW_REPO=$(cd "${SCRIPT_DIR}/../.."; pwd)
check_install gcloud
check_install kubectl
check_install ks

if [ -z "${CLIENT_ID}" ] || [ -z "${CLIENT_SECRET}" ]; then
  echo "CLIENT_ID and CLIENT_SECRET need to be set"
  exit 1
fi
# Name of the deployment
DEPLOYMENT_NAME=${DEPLOYMENT_NAME:-"kubeflow"}

# Kubeflow directories - Deployment Manager and Ksonnet App
KUBEFLOW_DM_DIR=${KUBEFLOW_DM_DIR:-"`pwd`/${DEPLOYMENT_NAME}_deployment_manager_configs"}
KUBEFLOW_KS_DIR=${KUBEFLOW_KS_DIR:-"`pwd`/${DEPLOYMENT_NAME}_ks_app"}
#TODO(ankushagarwal): Deal with the case when gcloud config is not set
# GCP Project
PROJECT=${PROJECT:-$(gcloud config get-value project 2>/dev/null)}
# GCP Zone
ZONE=${ZONE:-$(gcloud config get-value compute/zone 2>/dev/null)}
ZONE=${ZONE:-"us-central1-a"}
# Email for cert manager
EMAIL=${EMAIL:-$(gcloud config get-value account 2>/dev/null)}
# GCP Static IP Name
KUBEFLOW_IP_NAME=${KUBEFLOW_IP_NAME:-"${DEPLOYMENT_NAME}-ip"}
# Name of the endpoint
KUBEFLOW_ENDPOINT_NAME=${KUBEFLOW_ENDPOINT_NAME:-"${DEPLOYMENT_NAME}"}
# Complete hostname
KUBEFLOW_HOSTNAME=${KUBEFLOW_HOSTNAME:-"${KUBEFLOW_ENDPOINT_NAME}.endpoints.${PROJECT}.cloud.goog"}
# Whether to setup the project. Set to false to skip setting up the project.
SETUP_PROJECT=${SETUP_PROJECT:true}
# Namespace where kubeflow is deployed
K8S_NAMESPACE=${K8S_NAMESPACE:-"kubeflow"}
CONFIG_FILE=${CONFIG_FILE:-"cluster-kubeflow.yaml"}
PROJECT_NUMBER=`gcloud projects describe ${PROJECT} --format='value(project_number)'`
SA_EMAIL=${DEPLOYMENT_NAME}-admin@${PROJECT}.iam.gserviceaccount.com
USER_EMAIL=${DEPLOYMENT_NAME}-user@${PROJECT}.iam.gserviceaccount.com

if ${SETUP_PROJECT}; then
  # Enable GCloud APIs
  gcloud services enable deploymentmanager.googleapis.com \
                         servicemanagement.googleapis.com \
                         cloudresourcemanager.googleapis.com \
                         endpoints.googleapis.com \
                         iam.googleapis.com --project=${PROJECT}

  # Set IAM Admin Policy
  gcloud projects add-iam-policy-binding ${PROJECT} \
     --member serviceAccount:${PROJECT_NUMBER}@cloudservices.gserviceaccount.com \
     --role roles/resourcemanager.projectIamAdmin
else
  echo skipping project setup
fi

# Check if it already exists
set +e
gcloud deployment-manager --project=${PROJECT} deployments describe ${DEPLOYMENT_NAME}
exists=$?
set -e

cp -r "${SCRIPT_DIR}/deployment_manager_configs" "${KUBEFLOW_DM_DIR}"
cd "${KUBEFLOW_DM_DIR}"
# Set values in DM config file
sed -i.bak "s/zone: us-central1-a/zone: ${ZONE}/" "${KUBEFLOW_DM_DIR}/${CONFIG_FILE}"
sed -i.bak "s/users:/users: [\"user:${EMAIL}\"]/" "${KUBEFLOW_DM_DIR}/${CONFIG_FILE}"
sed -i.bak "s/ipName: kubeflow-ip/ipName: ${KUBEFLOW_IP_NAME}/" "${KUBEFLOW_DM_DIR}/${CONFIG_FILE}"
rm "${KUBEFLOW_DM_DIR}/${CONFIG_FILE}.bak"

if [ ${exists} -eq 0 ]; then
  echo ${DEPLOYMENT_NAME} exists
  gcloud deployment-manager --project=${PROJECT} deployments update ${DEPLOYMENT_NAME} --config=${CONFIG_FILE}
else
  # Run Deployment Manager
  gcloud deployment-manager --project=${PROJECT} deployments create ${DEPLOYMENT_NAME} --config=${CONFIG_FILE}
fi

# TODO(jlewi): We should name the secrets more consistently based on the service account name.
# We will need to update the component configs though
gcloud --project=${PROJECT} iam service-accounts keys create ${SA_EMAIL}.json --iam-account ${SA_EMAIL}
gcloud --project=${PROJECT} iam service-accounts keys create ${USER_EMAIL}.json --iam-account ${USER_EMAIL}

# Set credentials for kubectl context
gcloud --project=${PROJECT} container clusters get-credentials --zone=${ZONE} ${DEPLOYMENT_NAME}

# Make yourself cluster admin
kubectl create clusterrolebinding default-admin --clusterrole=cluster-admin --user=${EMAIL}

# The namespace kubeflow may not exist yet because the bootstrapper can't run until the admin-gcp-sa
# secret is created.
kubectl create namespace ${K8S_NAMESPACE}

# We want the secret name to be the same by default for all clusters so that users don't have to set it manually.
kubectl create secret generic --namespace=${K8S_NAMESPACE} admin-gcp-sa --from-file=admin-gcp-sa.json=./${SA_EMAIL}.json
kubectl create secret generic --namespace=${K8S_NAMESPACE} user-gcp-sa --from-file=user-gcp-sa.json=./${USER_EMAIL}.json
kubectl create secret generic --namespace=${K8S_NAMESPACE} kubeflow-oauth --from-literal=CLIENT_ID=${CLIENT_ID} --from-literal=CLIENT_SECRET=${CLIENT_SECRET}

# Install the GPU driver. It has not effect on non-GPU nodes.
kubectl apply -f https://raw.githubusercontent.com/GoogleCloudPlatform/container-engine-accelerators/stable/nvidia-driver-installer/cos/daemonset-preloaded.yaml

# Create the ksonnet app
cd $(dirname "${KUBEFLOW_KS_DIR}")
ks init $(basename "${KUBEFLOW_KS_DIR}")
cd "${KUBEFLOW_KS_DIR}"

ks env set default --namespace "${K8S_NAMESPACE}"
# Add the local registry
ks registry add kubeflow "${KUBEFLOW_REPO}/kubeflow"

# Install all required packages
ks pkg install kubeflow/core

# Generate all required components
ks generate kubeflow-core kubeflow-core --jupyterHubAuthenticator iap
ks generate cloud-endpoints cloud-endpoints
ks generate cert-manager cert-manager --acmeEmail=${EMAIL}
ks generate iap-ingress iap-ingress --ipName=${KUBEFLOW_IP_NAME} --hostname=${KUBEFLOW_HOSTNAME}

# Apply the components generated
ks apply default -c kubeflow-core
ks apply default -c cloud-endpoints
ks apply default -c cert-manager
ks apply default -c iap-ingress
