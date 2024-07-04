# Manually created jobs

While a `ControlledJob` is designed to start and stop a specified `Job` on a schedule automatically, you may sometimes want to start a job manually. For example, one day you may want to start the `Job` a little ahead of its scheduled time for some reason, or start it outside of its running schedule to fix some operational issue. This is analagous to the capability provided in `kubectl` to create a new `Job` based on the template provided in a `CronJob`:

```
kubectl create job --from=cronjob/foo foo-manual-0
```

Although of course we don't have first class support in `kubectl` for this!

The `controlled-job-operator` is designed with these kind of scenarios in mind. If it sees a manually created `Job` outside of the scheduled run times, it will assume the user has done that deliberately and leave the `Job` alone. At the next scheduled `stop` time any `Jobs` - including manually created `Jobs` - will be stopped. So you can safely create a new `Job` in the morning and be confident it will get stopped at the regular stop time.

A manually created `Job` can take the place of an automatically created `Job`:

- If you manually create a `Job` before the scheduled start time, then when the start time comes around the `controlled-job-operator` will take no action, as it sees there is already a `Job` running
- If you manually create a `Job` while an existing `Job` is already running the `controlled-job-operator` will 'adopt' that new `Job` as its chosen `Job` and will delete the old existing `Job`. This is one way to force a mid-day restart of your `ControlledJob`. In this case we recommend creating the `Job` in a suspended state, so that the `controlled-job-operator` can cleanly shut the old `Job` down before starting up your new one. Note: this assumes the job index of the job (the final digit in the `Job`'s name) is higher than the already running `Job`

## How to manually create a `Job` from a `ControlledJob`
We noted above that for `CronJobs` this functionality is provided by `kubectl`. For `ControlledJobs` we provide our own small CLI utility to do the heavy lifting of translating a `ControlledJob` into a valid `Job` object to create in K8s.

You pass the CLI some information about the `Job` (its scheduled start time, job run id etc) as command line args, and write a complete `ControlledJob` JSON manifest to STDIN and it will write out a valid `Job` definition to STDOUT, which you can then pass to `kubectl` to create the `Job` for you

Here's an example invocation:

```shell
$ kubectl get ctj controlledjob-sample-simple -o json | go run ./cli util generate-job --scheduled-at=2024-07-01T09:00:00Z --job-run-id=1 --manually-scheduled --start-suspended
{
  "kind": "Job",
  "apiVersion": "batch/v1",
  "metadata": {
    "name": "controlledjob-sample-simple-1719824400-1",
    "namespace": "my-namespace",
    "creationTimestamp": null,
    "labels": {
      "batch.gresearch.co.uk/controlled-job": "controlledjob-sample-simple"
    },
    "annotations": {
      "batch.gresearch.co.uk/is-manually-scheduled": "true",
      "batch.gresearch.co.uk/job-run-id": "1",
      "batch.gresearch.co.uk/job-template-hash": "e05d0f18ab184bdbabc771cca59810121a6aa26813cfdb87538a5698d8f38c3e",
      "batch.gresearch.co.uk/scheduled-at": "2024-07-01T09:00:00Z",
      "batch.gresearch.co.uk/timezone": "GMT"
    },
    "ownerReferences": [
      {
        "apiVersion": "batch.gresearch.co.uk/v1",
        "kind": "ControlledJob",
        "name": "controlledjob-sample-simple",
        "uid": "7a8d8847-0a08-4342-8d88-f07a8d9bf73a",
        "controller": true,
        "blockOwnerDeletion": true
      }
    ]
  },
  "spec": {
    "template": {
      "metadata": {
        "creationTimestamp": null,
        "labels": {
          "foo": "bar"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "hello",
            "image": "busybox",
            "args": [
              "/bin/sh",
              "-c",
              "while true\ndo\n  date\n  echo \"Hello from the Kubernetes cluster\"\n  sleep 5\ndone\n"
            ],
            "resources": {}
          }
        ],
        "restartPolicy": "OnFailure",
        "securityContext": {
          "runAsUser": 1000,
          "fsGroup": 1000
        }
      }
    },
    "suspend": true
  },
  "status": {}
}
```

And here's an example of directly creating a `Job` from the returned manifest:

```
$ kubectl get ctj controlledjob-sample-simple -o json | \
    go run ./cli util generate-job \
      --scheduled-at=2024-07-01T09:00:00Z \
      --job-run-id=1 \
      --manually-scheduled \
      --start-suspended | \
    kubectl apply -f -
job.batch/controlledjob-sample-simple-1719824400-1 created
```

The really key part of this tool is to add in the various annotations/metadata that are required on a `ControlledJob` `Job`. These include:

- an `OwnerReference` so that K8s knows it is owned by the `ControlledJob`
- details about its scheduled start time and job run id (an index which increases as new `Jobs` are created during the day)
- a hash of the template used to create it, so we can detect if the `Job` is out of date with the job template on the `ControlledJob`

Without this metadata, various parts of the `ControlledJob` logic will not work correctly, and either your manually created `Job` will be deleted immedaitely (if it has no scheduled time annotation), or will stay running past the stop time (if it has no ownership information).

