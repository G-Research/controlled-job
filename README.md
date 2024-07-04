# ControlledJob CRD

> For workloads which need to run only during particular periods of the day

A `ControlledJob` is a resource which specifies:

- the definition of a `Job` to be run (a plain K8s `JobSpec`)
- the schedule when we want that `Job` to be run. For example 'every weekday between 9am and 5pm, in the London timezone'

During the specified schedule, the `controlled-job-operator` will ensure that a `Job` object with a matching spec exists, and when the schedule says to stop, the `Job` is deleted.

Features:
- Control over what happens when the `JobSpec` specification on the `ControlledJob` changes while a `Job` is currently running. Either stop the old `Job` and start a new one with the new spec, or ignore it until the next scheduled run
- The ability to override the schedule manually. If a `Job` is manually created with the correct metadata, it will become managed by the matching `ControlledJob`. This allows use cases where the starting of a `Job` depends on external conditions (the successfuly completion of a batch job to prepare data for the Job perhaps) or when there's a need to start a `Job` earlier one day for some reason, but we still want the ongoing monitoring, restarting, and stopping to be handled according to the schedule
- Strong guarantees about exclusive running of the `Job`. If a `Job` is restarted for any reason, the `controlled-job-operator` will start it in a suspended state, and only unsuspend it when it's sure any previous `Job` can no longer be running.
- Pesimistic error handling. The system will not automatically retry failing `Jobs`, or restart `Jobs` that have exited cleanly during their scheduled time, to provide the user with the flexibility to choose how those cases are handled; settings on the `JobSpec` provided by Kubernetes already allow configuration of how to handle restarts and failures of a `Job` (eg retry up to 3 times before giving up). The logic from the `ControlledJob` side is simple: ensure a `Job` exists (in any state - starting, running, failed, succeeded) during the scheduled period, and is deleted outside of that period. The user can trigger a restart of a `ControlledJob` simply by deleting the current `Job`, which will trigger the `controlled-job-operator` to create a brand new `Job` in its place.
- Comprehensive `status` conditions, that can be used to drive alerting and health checks
- The ability to mutate the new `Job` specification at creation time. For example, a dynamic image tag lookup, or substituting the current date into an env var on the created `Pod`. Specify a URL to a service which should behave like a standard K8s mutating webhook for `Jobs` and it will be called before any `Job` is created.

## Example

```
apiVersion: batch.gresearch.co.uk/v1
kind: ControlledJob
metadata:
  name: controlledjob-sample
spec:

  # Timezone is any standard tz database timezone name
  # Optionally with an additional static offset (in seconds)
  timezone:
    name: "GMT"
    offset: 3600 # 1h, making the overall timezone 'GMT + 1h'

  # Any number of scheduled events. Each one is either 'start' or 'stop' and 
  # schedule can be timeOfDay & daysOfWeek, or a calid CRONTAB entry
  events:
    - action: "start"
      schedule:
        timeOfDay: "09:00"
        daysOfWeek: "MON-FRI"        
    - action: "stop"
      cronSchedule: "0 17 * * MON-FRI"

  # Template for the job to create. Any valid JobSpec is accepted
  jobTemplate:
    metadata:
      labels:
        foo: bar
    spec:
      backoffLimit: 3
      template:
        spec:
          containers:
          - name: hello
            image: busybox
            args:
            - /bin/sh
            - -c
            - date; echo Hello from my ControlledJob
          restartPolicy: OnFailure
```

## Developer guide
This operator is built on the standard [`controller-runtime` library](https://github.com/kubernetes-sigs/controller-runtime) using [Kubebuilder](https://book.kubebuilder.io/) and so should be familiar to anyone used to developing K8s controllers.

The main logic lives under [pkg/reconciliation](pkg/reconciliation) which is a good place to start reading.
