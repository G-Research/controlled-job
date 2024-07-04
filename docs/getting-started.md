# Getting started with ControlledJobs

We provide a Helm chart which should make deploying the CRD, RBAC roles and operator easy.

```
helm install controlled-job ./deploy/chart
```

Installing that chart should leave you with:
- the CRD for `ControlledJobs` added to your cluster
- a set of RBAC roles required to support the operation of the system
- a new namespace called `controlled-jobs` which houses the `controlled-job-operator` deployment. This will watch for changes to `ControlledJob` resources in the cluster and take action as required (eg starting up a new Job at the scheduled start time)

The chart provides options for customization, including

- Skipping creation of the namespace, service account, rbac roles, or the CRD itself, in case you want/need to create those externally (eg in a locked down environment where you have no permission to deploy cluster level resources)
- Adding extra labels to resources
- Mounting extra volumes to the deployment (eg if you have a custom CA bundle in your organisation)

See the chart [`values.yaml`](deploy/chart/values.yaml) for more information and available options

## Permissioning / RBAC

In order to do its job, the `controlled-job-operator` needs a set of permissions in your cluster, or at least in the namespaces where you want to deploy `ControlledJob` resources. These permissions are encapsulated in the [`controlledjob-manager-role` role](deploy/chart/templates/rbac/rbac.authorization.k8s.io_v1_clusterrole_controlledjob-manager-role.yaml). In brief it needs access to:

- `ControlledJob` resources (of course)
- `Job` resources - it needs to be able to create, delete, update and observe `Jobs`, as those are the resources which get created at the scheduled start times, and deleted at the stop times
- `Events` so it can record events that occur on a `ControlledJob`, which will appear when doing `kubectl describe ControlledJob`

If you enable the `rbac.clusterRoleBinding.create` flag when installing the Helm chart, then this role will be granted accross the whole cluster by default. If you'd like to opt-in per namespace, then add `--set rbac.clusterRoleBinding.create=false` when installing the chart, and manually create `RoleBindings` like the following in any opt-in namespace:

```
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: controlledjob-manager
  namespace: "... name of namespace to opt-in to ControlledJob support ..."
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: controlledjob-manager-role
subjects:
- kind: ServiceAccount
  name: controlled-job-operator # or whatever you set serviceAccount.name in the helm chart to
  namespace: controlled-job-operator # or whatever you set namespace.name in the helm chart to
```

## Testing it out

The [config/samples](config/samples) directory contains some example `ControlledJobs`. You can use `kubectl` to create one, and then observe its status, and the job it has created (if you're within the scheduled running time):

```shell
$ kubectl apply -f config/samples/simple.yaml                
controlledjob.batch.gresearch.co.uk/controlledjob-sample-simple created

$ kubectl get controlledjobs                 
NAME                                     IS RUNNING   SHOULD BE RUNNING   SUSPENDED   LAST SCHEDULED START TIME
controlledjob-sample-simple              true         true                false       59m

$ kubectl describe controlledjob controlledjob-sample-simple
...
Events:
  Type     Reason             Age                   From                     Message
  ----     ------             ----                  ----                     -------
  Normal   JobStarted         3s                    controlled-job-operator  Created job: controlledjob-sample-simple-1719997200-0

$ kubectl get jobs          
NAME                                       COMPLETIONS   DURATION   AGE
controlledjob-sample-simple-1719997200-0   0/1                      55s
```

## Where to go now?

Take a look into the `user-manual` folder for more docs about how to use and maintain the system