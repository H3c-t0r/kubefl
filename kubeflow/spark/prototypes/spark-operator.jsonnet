// @apiVersion 0.1
// @name io.ksonnet.pkg.spark-operator
// @param name string Name for the component
// @optionalParam namespace string default Namespace to use for the components. It is automatically inherited from the environment if not set.
// @optionalParam image string gcr.io/spark-operator/spark-operator:v2.3.1-v1alpha1-latest Image to use for spark operator

local k = import "k.libsonnet";
local spark = import "kubeflow/spark/all.libsonnet";

std.prune(
    k.core.v1.list.new(spark.all(params, params.name, env)))
