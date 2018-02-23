# Kubeflow

The Kubeflow project is dedicated to making machine learning on [Kubernetes](https://kubernetes.io/) simple, portable and scalable. Our goal is **not** to recreate other services, but to provide a straightforward way to train, test, and deploy best-of-breed open-source predictive models to diverse infrastructures. Anywhere you are running Kubernetes, you should be able to run Kubeflow.

****

Contained in this repository are manifests for creating:

* A [JupyterHub](https://jupyterhub.readthedocs.io/en/latest/) to create & manage interactive Jupyter notebooks
* A **TensorFlow Training Controller** that can be configured to use either CPUs or GPUs and dynamically adjusted to the size of a cluster with a single setting
* A **TensorFlow Serving** container to export trained TensorFlow models to Kubernetes

This document details the steps needed to run the Kubeflow project in any environment in which Kubernetes runs.

## Quick Links
* [Prow test dashboard](https://k8s-testgrid.appspot.com/sig-big-data)
* [Prow jobs dashboard](https://prow.k8s.io/?repo=kubeflow%2Fkubeflow)
* [Argo UI for E2E tests](http://testing-argo.kubeflow.io)

## The Kubeflow Mission

Our goal is to make scaling machine learning models and deploying them to production as simple as possible, by letting Kubernetes do what it's great at:
- Easy, repeatable, portable deployments on a diverse infrastructure (laptop <-> ML rig <-> training cluster <-> production cluster)
- Deploying and managing loosely-coupled microservices
- Scaling based on demand



Because ML practitioners use so many different types of tools, it's a key goal that you can customize the stack to whatever your requirements (within reason) and let the system take care of the "boring stuff." While we have started with a narrow set of technologies, we are working with many different projects to include additional tooling.

Ultimately, we want to have a set of simple manifests that give you an easy to use ML stack _anywhere_ Kubernetes is already running and can self configure based on the cluster it deploys into.


## Who should consider using Kubeflow?

Based on the current functionality you should consider using Kubeflow if:

  * You want to train/serve TensorFlow models in different environments (e.g. local, on prem, and cloud)
  * You want to use Jupyter notebooks to manage TensorFlow training jobs
       * kubeflow is particularly helpful if you want to launch training jobs that use more resources (more nodes or more GPUs) than your notebook.
  * You want to combine TensorFlow with other processes
       * For example, you may want to use [tensorflow/agents](https://github.com/tensorflow/agents) to run simulations to generate data for training reinforcement learning models.

This list is based ONLY on current capabilities. We are investing significant resources to expand the
functionality and actively soliciting help from companies and individuals interested in contributing (see [below](README.md#who-should-consider-contributing-to-kubeflow)).

## Setup

This documentation assumes you have a Kubernetes cluster already available.

If you need help setting up a Kubernetes cluster please refer to [Kubernetes Setup](https://kubernetes.io/docs/setup/).

If you want to use GPUs, be sure to follow the Kubernetes [instructions for enabling GPUs](https://kubernetes.io/docs/tasks/manage-gpus/scheduling-gpus/).

## Quick Start

### Requirements

  * ksonnet version [0.8.0](https://ksonnet.io/#get-started) or later.
  * Kubernetes >= 1.8 [see here](https://github.com/kubeflow/tf-operator#requirements)

### Steps

In order to quickly set up all components, execute the following commands:

```commandline
# Initialize a ksonnet APP
APP_NAME=my-kubeflow
ks init ${APP_NAME}
cd ${APP_NAME}

# Install Kubeflow components
ks registry add kubeflow github.com/kubeflow/kubeflow/tree/master/kubeflow
ks pkg install kubeflow/core
ks pkg install kubeflow/tf-serving
ks pkg install kubeflow/tf-job

# Deploy Kubeflow
NAMESPACE=kubeflow
kubectl create namespace ${NAMESPACE}
ks generate core kubeflow-core --name=kubeflow-core --namespace=${NAMESPACE}
ks apply default -c kubeflow-core
```


The above command sets up JupyterHub and a custom resource for running TensorFlow training jobs. Furthermore, the ksonnet packages
provide prototypes that can be used to configure TensorFlow jobs and deploy TensorFlow models.
Used together, these make it easy for a user go from training to serving using Tensorflow with minimal
effort in a portable fashion between different environments.

For more detailed instructions about how to use Kubeflow, please refer to the [user guide](user_guide.md).

## Troubleshooting
For detailed troubleshooting instructions, please refer to [this section of the user guide](user_guide.md#troubleshooting)

## Resources

* The [kubeflow user guide](user_guide.md) provides in-depth instructions for using Kubeflow
* Katacoda has produced a [self-paced scenario](https://www.katacoda.com/kubeflow) for learning and trying out Kubeflow


## Get Involved

* [Slack Channel](https://join.slack.com/t/kubeflow/shared_invite/enQtMjgyMzMxNDgyMTQ5LWUwMTIxNmZlZTk2NGU0MmFiNDE4YWJiMzFiOGNkZGZjZmRlNTExNmUwMmQ2NzMwYzk5YzQxOWQyODBlZGY2OTg)
* [Twitter](http://twitter.com/kubeflow)
* [Mailing List](https://groups.google.com/forum/#!forum/kubeflow-discuss)

In the interest of fostering an open and welcoming environment, we as contributors and maintainers pledge to making participation in our project and our community a harassment-free experience for everyone, regardless of age, body size, disability, ethnicity, gender identity and expression, level of experience, education, socio-economic status, nationality, personal appearance, race, religion, or sexual identity and orientation.

The Kubeflow community is guided by our [Code of Conduct](https://github.com/kubeflow/community/blob/master/CODE_OF_CONDUCT.md), which we encourage everybody to read before participating.



### Who should consider contributing to Kubeflow?

* Folks who want to add support for other ML frameworks (e.g. PyTorch, XGBoost, etc...)
* Folks who want to bring more Kubernetes magic to ML (e.g. ISTIO integration for prediction)
* Folks who want to make Kubeflow a richer ML platform (e.g. support for ML pipelines, hyperparameter tuning)
* Folks who want to tune Kubeflow for their particular Kubernetes distribution or Cloud
* Folks who want to write tutorials/blog posts showing how to use Kubeflow to solve ML problems
