{
  local util = import "kubeflow/core/util.libsonnet",
  new(_env, _params):: self + {
    local params = _env + _params + {
      namespace: if std.objectHas(_params, "namespace") && _params.namespace != "null" then
        _params.namespace else _env.namespace,
    },
    list:: util.list(self),

    PersistentVolume:: {
      apiVersion: "v1",
      kind: "PersistentVolume",
      metadata: {
        name: params.name,
        namespace: params.namespace,
      },
      spec: {
        capacity: {
          storage: params.storageCapacity,
        },
        accessModes: [
          "ReadWriteMany",
        ],
        nfs: {
          path: params.path,
          server: params.serverIP,
        },
      },
    },

    PersistentVolumeClaim:: {
      apiVersion: "v1",
      kind: "PersistentVolumeClaim",
      metadata: {
        name: params.name,
        namespace: params.namespace,
      },
      spec: {
        accessModes: [
          "ReadWriteMany",
        ],
        storageClassName: "",
        resources: {
          requests: {
            storage: params.storageCapacity,
          },
        },
      },
    },

    // Set 777 permissions on the GCFS NFS so that non-root users
    // like jovyan can use that NFS share
    GcfsPersmissions:: {
      apiVersion: "batch/v1",
      kind: "Job",
      metadata: {
        name: "set-gcfs-permissions",
        namespace: params.namespace,
      },
      spec: {
        template: {
          spec: {
            containers: [
              {
                name: "set-gcfs-permissions",
                image: params.image,
                command: [
                  "chmod",
                  "777",
                  "/kubeflow-gcfs",
                ],
                volumeMounts: [
                  {
                    mountPath: "/kubeflow-gcfs",
                    name: params.name,
                  },
                ],
              },
            ],
            restartPolicy: "OnFailure",
            volumes: [
              {
                name: params.name,
                persistentVolumeClaim: {
                  claimName: params.name,
                  readOnly: false,
                },
              },
            ],
          },
        },
      },
    },
  },
}
