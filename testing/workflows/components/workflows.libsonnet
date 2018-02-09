{
  // TODO(https://github.com/ksonnet/ksonnet/issues/222): Taking namespace as an argument is a work around for the fact that ksonnet
  // doesn't support automatically piping in the namespace from the environment to prototypes.

  // convert a list of two items into a map representing an environment variable
  listToMap:: function(v)
    {
      name: v[0],
      value: v[1],
    },

  // Function to turn comma separated list of prow environment variables into a dictionary.
  parseEnv:: function(v)
    local pieces = std.split(v, ",");
    if v != "" && std.length(pieces) > 0 then
      std.map(
        function(i) $.listToMap(std.split(i, "=")),
        std.split(v, ",")
      )
    else [],

  parts(namespace, name):: {
    // Workflow to run the e2e test.
    e2e(prow_env, bucket):
      // mountPath is the directory where the volume to store the test data
      // should be mounted.
      local mountPath = "/mnt/" + "test-data-volume";
      // testDir is the root directory for all data for a particular test run.
      local testDir = mountPath + "/" + name;
      // outputDir is the directory to sync to GCS to contain the output for this job.
      local outputDir = testDir + "/output";
      local artifactsDir = outputDir + "/artifacts";
      // Source directory where all repos should be checked out
      local srcRootDir = testDir + "/src";
      // The directory containing the kubeflow/kubeflow repo
      local srcDir = srcRootDir + "/kubeflow/kubeflow";
      local image = "gcr.io/mlkube-testing/test-worker:latest";
      // The name of the NFS volume claim to use for test files.
      local nfsVolumeClaim = "nfs-external";
      // The name to use for the volume to use to contain test data.
      local dataVolume = "kubeflow-test-volume";
      local kubeflowPy = srcDir;
      // The directory within the kubeflow_testing submodule containing
      // py scripts to use.      
      local kubeflowTestingPy = srcRootDir + "/kubeflow/testing/py";
      local tfOperatorRoot = srcRootDir + "/tensorflow/k8s";
      local tfOperatorPy = tfOperatorRoot;

      local project = "mlkube-testing";
      // GKE cluster to use
      local cluster = name;
      local zone = "us-east1-d";
      {
        // Build an Argo template to execute a particular command.
        // step_name: Name for the template
        // command: List to pass as the container command.
        buildTemplate(step_name, command):: {
          name: step_name,
          container: {
            command: command,
            image: image,
            env: [
              {
                // Add the source directories to the python path.
                name: "PYTHONPATH",
                value: kubeflowPy + ":" + kubeflowTestingPy + ":" + tfOperatorPy,
              },
              {
                name: "GOOGLE_APPLICATION_CREDENTIALS",
                value: "/secret/gcp-credentials/key.json",
              },
              {
                name: "GIT_TOKEN",
                valueFrom: {
                  secretKeyRef: {
                    name: "github-token",
                    key: "github_token",
                  },
                },
              },
            ] + prow_env,
            volumeMounts: [
              {
                name: dataVolume,
                mountPath: mountPath,
              },
              {
                name: "github-token",
                mountPath: "/secret/github-token",
              },
              {
                name: "gcp-credentials",
                mountPath: "/secret/gcp-credentials",
              },
            ],
          },
        },  // buildTemplate

        apiVersion: "argoproj.io/v1alpha1",
        kind: "Workflow",
        metadata: {
          name: name,
          namespace: namespace,
        },
        // TODO(jlewi): Use OnExit to run cleanup steps.
        spec: {
          entrypoint: "e2e",
          volumes: [
            {
              name: "github-token",
              secret: {
                secretName: "github-token",
              },
            },
            {
              name: "gcp-credentials",
              secret: {
                secretName: "kubeflow-testing-credentials",
              },
            },
            {
              name: dataVolume,
              persistentVolumeClaim: {
                claimName: nfsVolumeClaim,
              },
            },
          ],  // volumes
          // onExit specifies the template that should always run when the workflow completes.
          onExit: "exit-handler",
          templates: [
            {
              name: "e2e",
              steps: [
                [{
                  name: "checkout",
                  template: "checkout",
                }],
                [
                  {
                    name: "setup",
                    template: "setup",
                  },
                  {
                    name: "create-pr-symlink",
                    template: "create-pr-symlink",
                  },
                ],
                [{
                    name: "tfjob-test",
                    template: "tfjob-test",
                  },                
                ],
              ],
            },
            {
              name: "exit-handler",
              steps: [
                [
                  {
                    name: "teardown",
                    template: "teardown",
                  },
                ],
                [{
                  name: "copy-artifacts",
                  template: "copy-artifacts",
                }],
              ],
            },
            {
              name: "checkout",
              container: {
                command: [
                  "/usr/local/bin/checkout.sh",
                ],
                args: [
                  srcRootDir,
                ],
                env: prow_env + [{
                  "name": "EXTRA_REPOS",
                  // TODO(jlewi): Once tensorflow/k8s#374 is submitted pin
                  // to tensorflow/k8s HEAD
                  "value": "tensorflow/k8s@HEAD:374;kubeflow/testing@HEAD",
                }],
                image: image,
                volumeMounts: [
                  {
                    name: dataVolume,
                    mountPath: mountPath,
                  },
                ],
              },
            },  // checkout
            $.parts(namespace, name).e2e(prow_env, bucket).buildTemplate("setup", [
              "python",
              "-m",
              "testing.test_deploy",              
              "--project=mlkube-testing",
              "--cluster=kubeflow-testing",
              "--namespace=" + namespace,
              "--zone=us-east1-d",
              "--github_token=$(GIT_TOKEN)",
              "--test_dir=" + testDir,
              "--artifacts_dir=" + artifactsDir,
              "setup",
            ]),  // setup
            $.parts(namespace, name).e2e(prow_env, bucket).buildTemplate("teardown", [
              "python",
              "-m",
              "testing.test_deploy",
              "--project=mlkube-testing",
              "--cluster=kubeflow-testing",
              "--namespace=" + namespace,
              "--zone=us-east1-d",
              "--github_token=$(GIT_TOKEN)",
              "--test_dir=" + testDir,
              "--artifacts_dir=" + artifactsDir,
              "teardown",
            ]),  // teardown
            $.parts(namespace, name).e2e(prow_env, bucket).buildTemplate("create-pr-symlink", [
              "python",
              "-m",
              "kubeflow.testing.prow_artifacts",
              "--artifacts_dir=" + outputDir,
              "create_pr_symlink",
              "--bucket=" + bucket,
            ]),  // create-pr-symlink
            $.parts(namespace, name).e2e(prow_env, bucket).buildTemplate("copy-artifacts", [
              "python",
              "-m",
              "kubeflow.testing.prow_artifacts",
              "--artifacts_dir=" + outputDir,
              "copy_artifacts",
              "--bucket=" + bucket,
            ]),  // copy-artifacts
            $.parts(namespace, name).e2e(prow_env, bucket).buildTemplate("tfjob-test", [
              "python",
              "-m",
              "py.test_runner",
              "test",
              "--cluster=" + cluster,
              "--zone=" + zone,
              "--project=" + project,
              "--app_dir=" + srcDir + "/test/workflows",
              "--component=simple_tfjob",
              "--params=name=simple-tfjob,namespace=" + namespace,
              "--junit_path=" + artifactsDir + "/junit_e2e.xml",
            ]),  // run tests
          ],  // templates
        },
      },  // e2e
  },  // parts
}
