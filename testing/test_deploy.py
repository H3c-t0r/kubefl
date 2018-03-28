#!/usr/bin/env python

# Copyright 2018 The Kubeflow Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
"""Test deploying Kubeflow.

Requirements:
  This project assumes the py directory in github.com/kubeflow/tf-operator corresponds
  to a top level Python package on the Python path.

  TODO(jlewi): Come up with a better story for how we reuse the py package
  in kubeflow/tf-operator. We should probably turn that into a legit Python pip
  package that is built and released as part of the kubeflow/tf-operator project.
"""

import argparse
import datetime
import json
import logging
import os
import shutil
import tempfile
import uuid

import requests
import yaml
from googleapiclient import discovery, errors
from kubernetes import client as k8s_client
from kubernetes.client import rest
from kubernetes.config import kube_config
from oauth2client.client import GoogleCredentials

from kubeflow.testing import test_util, util  # pylint: disable=no-name-in-module
from testing import vm_util


def _setup_test(api_client, run_label):
  """Create the namespace for the test.

  Returns:
    test_dir: The local test directory.
  """

  api = k8s_client.CoreV1Api(api_client)
  namespace = k8s_client.V1Namespace()
  namespace.api_version = "v1"
  namespace.kind = "Namespace"
  namespace.metadata = k8s_client.V1ObjectMeta(
    name=run_label, labels={
      "app": "kubeflow-e2e-test",
    })

  try:
    logging.info("Creating namespace %s", namespace.metadata.name)
    namespace = api.create_namespace(namespace)
    logging.info("Namespace %s created.", namespace.metadata.name)
  except rest.ApiException as e:
    if e.status == 409:
      logging.info("Namespace %s already exists.", namespace.metadata.name)
    else:
      raise

  return namespace


def create_k8s_client(_):
  # DO NOT SUBMIT.
  # util.load_kube_config()

  # Create an API client object to talk to the K8s master.
  api_client = k8s_client.ApiClient()

  return api_client


# TODO(jlewi): We should make this a reusable function in kubeflow/testing
# because we will probably want to use it in other places as well.
def setup_kubeflow_ks_app(args, api_client):
  """Create a ksonnet app for Kubeflow"""
  if not os.path.exists(args.test_dir):
    os.makedirs(args.test_dir)

  logging.info("Using test directory: %s", args.test_dir)

  namespace_name = args.namespace

  namespace = _setup_test(api_client, namespace_name)
  logging.info("Using namespace: %s", namespace)
  if args.github_token:
    logging.info("Setting GITHUB_TOKEN to %s.", args.github_token)
    # Set a GITHUB_TOKEN so that we don't rate limited by GitHub;
    # see: https://github.com/ksonnet/ksonnet/issues/233
    os.environ["GITHUB_TOKEN"] = args.github_token

  if not os.getenv("GITHUB_TOKEN"):
    logging.warning("GITHUB_TOKEN not set; you will probably hit Github API "
                    "limits.")
  # Initialize a ksonnet app.
  app_name = "kubeflow-test-" + uuid.uuid4().hex[0:4]
  util.run(
    [
      "ks",
      "init",
      app_name,
    ], cwd=args.test_dir)

  app_dir = os.path.join(args.test_dir, app_name)

  kubeflow_registry = "github.com/kubeflow/kubeflow/tree/master/kubeflow"
  util.run(
    ["ks", "registry", "add", "kubeflow", kubeflow_registry], cwd=app_dir)

  # Install required packages
  packages = ["kubeflow/core", "kubeflow/tf-serving", "kubeflow/tf-job"]

  for p in packages:
    util.run(["ks", "pkg", "install", p], cwd=app_dir)

  # Delete the vendor directory and replace with a symlink to the src
  # so that we use the code at the desired commit.
  target_dir = os.path.join(app_dir, "vendor", "kubeflow")

  logging.info("Deleting %s", target_dir)
  shutil.rmtree(target_dir)

  REPO_ORG = "kubeflow"
  REPO_NAME = "kubeflow"
  REGISTRY_PATH = "kubeflow"
  source = os.path.join(args.test_dir, "src", REPO_ORG, REPO_NAME,
                        REGISTRY_PATH)
  logging.info("Creating link %s -> %s", target_dir, source)
  os.symlink(source, target_dir)

  return app_dir


def get_gke_credentials(args):
  """Configure kubeconfig to talk to the supplied GKE cluster."""
  config_file = os.path.expanduser(kube_config.KUBE_CONFIG_DEFAULT_LOCATION)
  logging.info("Using Kubernetes config file: %s", config_file)
  project = args.project
  cluster_name = args.cluster
  zone = args.zone
  logging.info("Using cluster: %s in project: %s in zone: %s", cluster_name,
               project, zone)
  # Print out config to help debug issues with accounts and
  # credentials.
  util.run(["gcloud", "config", "list"])
  util.configure_kubectl(project, zone, cluster_name)

  # TODO(jlewi): If GOOGLE_APPLICATION_CREDENTIALS is set then I think
  # we want to modify the KUBECONFIG file to remove the GCP commands.
  # This will allow it to be truly headless and not require gcloud. 
  # More importantly, kubectl will properly attach auth.info scope so
  # that RBAC rules can be applied to the email and not the id.
  # See https://github.com/kubernetes/kubernetes/pull/58141
  
  # TODO(jlewi): Make this a flag.

  logging.info("Modifying kubeconfig %s", config_file)
  with open(config_file, "r") as hf:
    config = yaml.load(hf)
    
  for user in config["users"]:
    auth_provider = user.get("user", {}).get("auth-provider", {})
    if auth_provider.get("name") != "gcp":
      continue
    logging.info("Modifying user %s which has gcp auth provider", user["name"])
    if "config" in auth_provider:
      logging.info("Deleting config from user %s", user["name"])
      del auth_provider["config"]

  logging.info("Writing update kubeconfig:\n %s", yaml.dump(config))  
  with open(config_file, "w") as hf:
    yaml.dump(config, hf)
    
def deploy_kubeflow(args):
  """Deploy Kubeflow."""
  api_client = create_k8s_client(args)
  app_dir = setup_kubeflow_ks_app(args, api_client)

  namespace = args.namespace
  # TODO(jlewi): We don't need to generate a core component if we are
  # just deploying TFServing. Might be better to refactor this code.
  # Deploy Kubeflow
  util.run(
    [
      "ks", "generate", "core", "kubeflow-core", "--name=kubeflow-core",
      "--namespace=" + namespace
    ],
    cwd=app_dir)

  apply_command = [
    "ks",
    "apply",    
    "default",    
    "-c",
    "kubeflow-core",
  ]

  if args.as_gcloud_user:
    account = get_gcp_identity()
    logging.info("Impersonate %s", account)

    # If we don't use --as to impersonate the service account then we
    # observe RBAC errors when doing certain operations. The problem appears
    # to be that we end up using the in cluster config (e.g. pod service account)
    # and not the GCP service account which has more privileges.
    apply_command.append("--as=" + account)
  util.run(apply_command, cwd=app_dir)

  # Verify that the TfJob operator is actually deployed.
  tf_job_deployment_name = "tf-job-operator"
  logging.info("Verifying TfJob controller started.")
  util.wait_for_deployment(api_client, namespace, tf_job_deployment_name)

  # Verify that JupyterHub is actually deployed.
  jupyter_name = "tf-hub"
  logging.info("Verifying TfHub started.")
  util.wait_for_statefulset(api_client, namespace, jupyter_name)


def deploy_model(args):
  """Deploy a TF model using the TF serving component."""
  api_client = create_k8s_client(args)
  app_dir = setup_kubeflow_ks_app(args, api_client)

  component = "modelServer"
  logging.info("Deploying tf-serving.")
  generate_command = ["ks", "generate", "tf-serving", component]

  util.run(generate_command, cwd=app_dir)

  params = {}
  for pair in args.params.split(","):
    k, v = pair.split("=", 1)
    params[k] = v

  if "namespace" not in params:
    raise ValueError("namespace must be supplied via --params.")
  namespace = params["namespace"]

  ks_deploy(app_dir, component, params, env=None, account=None)

  core_api = k8s_client.CoreV1Api(api_client)
  deploy = core_api.read_namespaced_service(args.deploy_name, args.namespace)
  cluster_ip = deploy.spec.cluster_ip

  if not cluster_ip:
    raise ValueError("inception service wasn't assigned a cluster ip.")
  util.wait_for_deployment(
    api_client, namespace, args.deploy_name, timeout_minutes=10)
  logging.info("Verified TF serving started.")


def teardown(args):
  # Delete the namespace
  logging.info("Deleting namespace %s", args.namespace)
  api_client = create_k8s_client(args)
  core_api = k8s_client.CoreV1Api(api_client)
  core_api.delete_namespace(args.namespace, {})


def determine_test_name(args):
  if args.deploy_name:
    return args.func.__name__ + "-" + args.deploy_name
  return args.func.__name__


# TODO(jlewi): We should probably make this a generic function in
# kubeflow.testing.`
def wrap_test(args):
  """Run the tests given by args.func and output artifacts as necessary.
  """
  test_name = determine_test_name(args)
  test_case = test_util.TestCase()
  test_case.class_name = "KubeFlow"
  test_case.name = "deploy-kubeflow-" + test_name
  try:

    def run():
      args.func(args)

    test_util.wrap_test(run, test_case)
  finally:

    junit_path = os.path.join(args.artifacts_dir,
                              "junit_kubeflow-deploy-{0}.xml".format(test_name))
    logging.info("Writing test results to %s", junit_path)
    test_util.create_junit_xml_file([test_case], junit_path)


# TODO(jlewi): We should probably make this a reusable function since a
# lot of test code code use it.
def ks_deploy(app_dir, component, params, env=None, account=None):
  """Deploy the specified ksonnet component.
  Args:
    app_dir: The ksonnet directory
    component: Name of the component to deployed
    params: A dictionary of parameters to set; can be empty but should not be
      None.
    env: (Optional) The environment to use, if none is specified a new one
      is created.
    account: (Optional) The account to use.
  Raises:
    ValueError: If input arguments aren't valid.
  """
  if not component:
    raise ValueError("component can't be None.")

  # TODO(jlewi): It might be better if the test creates the app and uses
  # the latest stable release of the ksonnet configs. That however will cause
  # problems when we make changes to the TFJob operator that require changes
  # to the ksonnet configs. One advantage of checking in the app is that
  # we can modify the files in vendor if needed so that changes to the code
  # and config can be submitted in the same pr.
  now = datetime.datetime.now()
  if not env:
    env = "e2e-" + now.strftime("%m%d-%H%M-") + uuid.uuid4().hex[0:4]

  logging.info("Using app directory: %s", app_dir)

  util.run(["ks", "env", "add", env], cwd=app_dir)

  for k, v in params.iteritems():
    util.run(
      ["ks", "param", "set", "--env=" + env, component, k, v], cwd=app_dir)

  apply_command = ["ks", "apply", env, "-c", component]
  if account:
    apply_command.append("--as=" + account)
  util.run(apply_command, cwd=app_dir)


def modify_minikube_config(config_path, certs_dir):
  """Modify the kube config file used with minikube.

  This function changes the location of the certificates to certs_dir.

  Args:
    config_path: The path of the Kubernetes config file.
    certs_dir: The directory where the certs to use with minikube are stored.
  """
  with open(config_path, "r") as hf:
    config = yaml.load(hf)

  for cluster in config["clusters"]:
    authority = cluster["cluster"]["certificate-authority"]
    authority = os.path.join(certs_dir, os.path.basename(authority))
    cluster["cluster"]["certificate-authority"] = authority

    for user in config["users"]:
      for k in ["client-certificate", "client-key"]:
        user["user"][k] = os.path.join(certs_dir,
                                       os.path.basename(user["user"][k]))

  logging.info("Updating path of certificates in %s", config_path)
  with open(config_path, "w") as hf:
    yaml.dump(config, hf)


def deploy_minikube(args):
  """Create a VM and setup minikube."""

  credentials = GoogleCredentials.get_application_default()
  gce = discovery.build("compute", "v1", credentials=credentials)
  instances = gce.instances()
  body = {
    "name":
    args.vm_name,
    "machineType":
    "zones/{0}/machineTypes/n1-standard-16".format(args.zone),
    "disks": [
      {
        "boot": True,
        "initializeParams": {
          "sourceImage":
          "projects/ubuntu-os-cloud/global/images/family/ubuntu-1604-lts",
          "diskSizeGb":
          100,
          "autoDelete":
          True,
        },
      },
    ],
    "networkInterfaces": [
      {
        "accessConfigs": [
          {
            "name": "external-nat",
            "type": "ONE_TO_ONE_NAT",
          },
        ],
        "network": "global/networks/default",
      },
    ],
  }
  request = instances.insert(project=args.project, zone=args.zone, body=body)
  try:
    request.execute()
  except errors.HttpError as e:
    if not e.content:
      raise
    content = json.loads(e.content)
    # TODO(jlewi): We can get this error if the disk exists but not the VM.
    # If the disk exists but not the vm and we keep going we will have a
    # problem. However, that should be extremely unlikely now
    # that we set auto-delete on the disk to true.
    if content.get("error", {}).get("code") == requests.codes.CONFLICT:
      logging.warning("VM %s already exists in zone %s in project %s ",
                      args.vm_name, args.zone, args.project)
    else:
      raise

  # Locate the install minikube script.
  install_script = os.path.join(
    os.path.dirname(__file__), "install_minikube.sh")

  if not os.path.exists(install_script):
    logging.error("C %s", install_script)

  vm_util.wait_for_vm(args.project, args.zone, args.vm_name)
  vm_util.execute_script(args.project, args.zone, args.vm_name, install_script)

  # Copy the .kube and .minikube files to test_dir
  for target in ["~/.kube"]:
    full_target = "{0}:{1}".format(args.vm_name, target)
    logging.info("Copying %s to %s", target, args.test_dir)
    util.run([
      "gcloud", "compute", "--project=" + args.project, "scp", "--recurse",
      full_target, args.test_dir, "--zone=" + args.zone
    ])

  # The .minikube directory contains some really large ISO and other files that we don't need; so we
  # only copy the files we need.
  minikube_dir = os.path.join(args.test_dir, ".minikube")
  if not os.path.exists(minikube_dir):
    os.makedirs(minikube_dir)

  for target in ["~/.minikube/*.crt", "~/.minikube/client.key"]:
    full_target = "{0}:{1}".format(args.vm_name, target)
    logging.info("Copying %s to %s", target, minikube_dir)
    util.run([
      "gcloud", "compute", "--project=" + args.project, "scp", "--recurse",
      full_target, minikube_dir, "--zone=" + args.zone
    ])

  config_path = os.path.join(args.test_dir, ".kube", "config")
  modify_minikube_config(config_path, minikube_dir)


def teardown_minikube(args):
  """Delete the VM used for minikube."""

  credentials = GoogleCredentials.get_application_default()
  gce = discovery.build("compute", "v1", credentials=credentials)
  instances = gce.instances()

  request = instances.delete(
    project=args.project, zone=args.zone, instance=args.vm_name)

  request.execute()

def get_gcp_identity():
  identity = util.run_and_output(["gcloud", "config", "get-value", "account"])
  logging.info("Current GCP account: %s", identity)
  return identity
  
def main():  # pylint: disable=too-many-locals,too-many-statements
  logging.getLogger().setLevel(logging.INFO)  # pylint: disable=too-many-locals
  # create the top-level parser
  parser = argparse.ArgumentParser(description="Test Kubeflow E2E.")

  parser.add_argument(
    "--test_dir",
    default="",
    type=str,
    help="Directory to use for all the test files. If not set a temporary "
    "directory is created.")

  parser.add_argument(
    "--artifacts_dir",
    default="",
    type=str,
    help="Directory to use for artifacts that should be preserved after "
    "the test runs. Defaults to test_dir if not set.")

  parser.add_argument("--as_gcloud_user", dest="as_gcloud_user", 
                      action="store_true", 
                      help=("Impersonate the user corresponding to the gcloud "
                            "command with kubectl and ks."))
  parser.add_argument("--no-as_gcloud_user", dest="as_gcloud_user", 
                      action="store_false")
  parser.set_defaults(as_gcloud_user=False)
  
  # TODO(jlewi): This should not be a global flag.
  parser.add_argument(
    "--project", default=None, type=str, help="The project to use.")

  # TODO(jlewi): This should not be a global flag.
  parser.add_argument(
    "--namespace", default=None, type=str, help=("The namespace to use."))

  parser.add_argument(
    "--github_token",
    default=None,
    type=str,
    help=("The GitHub API token to use. This is needed since ksonnet uses the "
          "GitHub API and without it we get rate limited. For more info see: "
          "https://github.com/ksonnet/ksonnet/blob/master/docs"
          "/troubleshooting.md. Can also be set using environment variable "
          "GITHUB_TOKEN."))

  parser.add_argument(
    "--deploy_name", default="", type=str, help="The name of the deployment.")

  subparsers = parser.add_subparsers()

  parser_gke = subparsers.add_parser(
    "get_gke_credentials", help="Configure kubectl for a GKE cluster.")

  parser_gke.set_defaults(func=get_gke_credentials)

  parser_gke.add_argument(
    "--cluster",
    default=None,
    type=str,
    help=("The name of the cluster. If not set assumes the "
          "script is running in a cluster and uses that cluster."))

  parser_gke.add_argument(
    "--zone", default="us-east1-d", type=str, help="The zone for the cluster.")

  parser_teardown = subparsers.add_parser(
    "teardown", help="teardown the test infrastructure.")

  parser_teardown.set_defaults(func=teardown)

  parser_kubeflow = subparsers.add_parser(
    "deploy_kubeflow", help="Deploy kubeflow.")

  parser_kubeflow.set_defaults(func=deploy_kubeflow)

  parser_tf_serving = subparsers.add_parser(
    "deploy_model", help="Deploy a TF serving model.")

  parser_tf_serving.set_defaults(func=deploy_model)

  parser_tf_serving.add_argument(
    "--params",
    default="",
    type=str,
    help=("Comma separated list of parameters to set on the model."))

  parser_minikube = subparsers.add_parser(
    "deploy_minikube", help="Setup a K8s cluster on minikube.")

  parser_minikube.set_defaults(func=deploy_minikube)

  parser_minikube.add_argument(
    "--vm_name", required=True, type=str, help="The name of the VM to use.")

  parser_minikube.add_argument(
    "--zone", default="us-east1-d", type=str, help="The zone for the cluster.")

  parser_teardown_minikube = subparsers.add_parser(
    "teardown_minikube", help="Delete the VM running minikube.")

  parser_teardown_minikube.set_defaults(func=teardown_minikube)

  parser_teardown_minikube.add_argument(
    "--zone", default="us-east1-d", type=str, help="The zone for the cluster.")

  parser_teardown_minikube.add_argument(
    "--vm_name", required=True, type=str, help="The name of the VM to use.")

  args = parser.parse_args()

  if not args.test_dir:
    logging.info("--test_dir not set; using a temporary directory.")

    now = datetime.datetime.now()
    label = "test_deploy-" + now.strftime("%m%d-%H%M-") + uuid.uuid4().hex[0:4]

    # Create a temporary directory for this test run
    args.test_dir = os.path.join(tempfile.gettempdir(), label)

  if not args.artifacts_dir:
    args.artifacts_dir = args.test_dir

  test_log = os.path.join(
    args.artifacts_dir, "logs",
    "test_deploy." + args.func.__name__ + args.deploy_name + ".log.txt")
  if not os.path.exists(os.path.dirname(test_log)):
    os.makedirs(os.path.dirname(test_log))

  # TODO(jlewi): We should make this a util routine in kubeflow.testing.util
  # Setup a logging file handler. This way we can upload the log outputs
  # to gubernator.
  root_logger = logging.getLogger()

  file_handler = logging.FileHandler(test_log)
  root_logger.addHandler(file_handler)
  # We need to explicitly set the formatter because it will not pick up
  # the BasicConfig.
  formatter = logging.Formatter(
    fmt=("%(levelname)s|%(asctime)s"
         "|%(pathname)s|%(lineno)d| %(message)s"),
    datefmt="%Y-%m-%dT%H:%M:%S")
  file_handler.setFormatter(formatter)
  logging.info("Logging to %s", test_log)
  util.run(["ks", "version"])

  util.maybe_activate_service_account()
  config_file = os.path.expanduser(kube_config.KUBE_CONFIG_DEFAULT_LOCATION)

  # Print out the config to help debugging.
  output = util.run_and_output(["gcloud", "config", "config-helper"])
  logging.info("gcloud config: \n%s", output)
  wrap_test(args)


if __name__ == "__main__":
  logging.basicConfig(
    level=logging.INFO,
    format=('%(levelname)s|%(asctime)s'
            '|%(pathname)s|%(lineno)d| %(message)s'),
    datefmt='%Y-%m-%dT%H:%M:%S',)
  logging.getLogger().setLevel(logging.INFO)
  main()
