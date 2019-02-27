# -*- coding: utf-8 -*-
import json
import base64
import os
from kubernetes import client, config
from kubernetes.config import ConfigException
from kubernetes.client.rest import ApiException
from kubeflow.rokui.utils import parse_user_template

try:
  # Load configuration inside the Pod
  config.load_incluster_config()
except ConfigException:
  # Load configuration for testing
  config.load_kube_config()

# Create the Apis
v1_core = client.CoreV1Api()
custom_api = client.CustomObjectsApi()


def parse_error(e):
  try:
    err = json.loads(e.body)['message']
  except json.JSONDecodeError:
    err = str(e)
  except KeyError:
    err = str(e)

  return err


def get_secret(nm, ns):
  return v1_core.read_namespaced_secret(nm, ns)


def get_rok_token(ns):
  """Retrieve the token to authenticate with Rok."""
  secret = None
  nm = ''
  if os.environ.get('ROK_SECRET_NAME') != 'null':
    nm = os.environ.get('ROK_SECRET_NAME')
    nm = parse_user_template(nm)
  
  try:
    secret = v1_core.read_namespaced_secret(name=nm, namespace=ns)
  except ApiException:
    return ''

  token = secret.data.get('token', '')

  return base64.b64decode(token).decode('utf-8')


def get_namespaces():
  nmsps = v1_core.list_namespace()
  return [ns.metadata.name for ns in nmsps.items]


def get_notebooks(ns):
  custom_api = client.CustomObjectsApi()

  notebooks = \
      custom_api.list_namespaced_custom_object("kubeflow.org", "v1alpha1",
                                               ns, "notebooks")
  return [nb['metadata']['name'] for nb in notebooks['items']]


def delete_notebook(nb, ns):
  body = client.V1DeleteOptions()

  return \
      custom_api.delete_namespaced_custom_object("kubeflow.org", "v1alpha1",
                                                 ns, "notebooks", nb, body)


def create_notebook(body):
  ns = body['metadata']['namespace']
  return \
      custom_api.create_namespaced_custom_object("kubeflow.org", "v1alpha1",
                                                 ns, "notebooks", body)


def create_pvc(body):
  ns = body.metadata.namespace
  return v1_core.create_namespaced_persistent_volume_claim(ns, body)
