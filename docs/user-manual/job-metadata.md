# Job metadata

To help the operation of the `ControlledJob` scheduler, various bits of metadata get added to `Jobs` that it creates:

## Owner reference
We set the K8s `OwnerReference` of a `Job` to the `ControlledJob` which created it. This tells K8s that this isn't just a standalone `Job` and so it will not automatically garbage collect the `Job` until the `ControlledJob` deletes it. It also means tools like ArgoCD which visually show the hierarchy of K8s resources will show the `Job` as owned by the `ControlledJob`

```
ownerReferences:
  - apiVersion: batch.gresearch.co.uk/v1
    blockOwnerDeletion: true
    controller: true
    kind: ControlledJob
    name: controlledjob-sample-simple
    uid: 7a8d8847-0a08-4342-8d88-f07a8d9bf73a
```

## Labels

As well as any labels set in the `jobTemplate.metadata.labels`, we add a label with the name of the `ControlledJob`:

```
labels:
  batch.gresearch.co.uk/controlled-job: controlledjob-sample-simple
```

This makes it easy to find all `Jobs` owned by a given `ControlledJob`:

```
kubectl get jobs -lbatch.gresearch.co.uk/controlled-job=controlledjob-sample-simple 
NAME                                       COMPLETIONS   DURATION   AGE
controlledjob-sample-simple-1720087200-1   0/1           4m10s      10m
```

## Annotations

As well as any annotations set in the `jobTemplate.metadata.annotations`, we add the following bits of metadata to record various properties of the `Job`:

```
batch.gresearch.co.uk/is-manually-scheduled: "true"
batch.gresearch.co.uk/job-run-id: "1"
batch.gresearch.co.uk/job-template-hash: e05d0f18ab184bdbabc771cca59810121a6aa26813cfdb87538a5698d8f38c3e
batch.gresearch.co.uk/scheduled-at: "2024-07-04T10:00:00Z"
batch.gresearch.co.uk/timezone: GMT
```

- `batch.gresearch.co.uk/scheduled-at`: A timestamp recording the time the `Job` was scheduled to start at, which may be different to the `creationTimestamp`, which is when the K8s resource was created. For example, the `scheduled-at` time may be 9am (corresponding to a `start` event at 9am that day), but the `creationTimestamp` may be a few seconds after that if the `controlled-job-operator` took a little time to process the start event.
- `batch.gresearch.co.uk/job-run-id`: it's possible that during the course of one scheduled run period (ie between a start and a stop time), more than one `Job` may be created. If the spec changes are `recreate` is set as the `specChangePolicy` then a new `Job` will be created to replace the old one. `job-run-id` is a simple 0-based index of `Jobs` during the course of one scheduled run period, to disambiguate these different runs. **Note:** do not rely on this number strictly increasing. If you were to delete a running `Job` then the `controlled-job-operator` may recreate the `Job` with the same `job-run-id` (as it can't see the deleted `Job` to know there had been a previous run that day)
- `batch.gresearch.co.uk/job-template-hash`: In order to keep track of whether the currently running job matches the desired job spec set on the `ControlledJob`, we record a SHA256 hash of the `jobTemplate` at the point the `Job` was created, so it can later be compared with the latest `jobTemplate`
- `batch.gresearch.co.uk/is-manually-scheduled`: should be set on any `Job` which has been [manually created](docs/user-manual/manually-created-jobs.md). This tells the `controlled-job-operator` not to delete this `Job` until the next stop time.
- `batch.gresearch.co.uk/timezone`: records the timezone on the `ControlledJob` at the time this `Job` was created
