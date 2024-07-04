# How it works

This page sets out how the `ControlledJob` behaves, and how it responds to different events and conditions. You can see this logic in code in the [`pkg/reconciliation/decision.go`](pkg/reconciliation/decision.go)
 file.

This includes quite a deep-dive into how things work under the hood, but we think it's important to understand exactly how things operate to have a good idea how to handle issues and to predict how different setups will behave.

## The basics

A `ControlledJob` defines both:

- a list of events which define a schedule for when it should be running (in a given timezone). For example the following says to start at 9am every Monday to Friday, and stop at 5pm every Monday to Friday
```
  events:
    - action: "start"
      schedule:
        timeOfDay: "09:00"
        daysOfWeek: "MON-FRI"
    - action: "stop"
      schedule:
        timeOfDay: "17:00"
        daysOfWeek: "MON-FRI"
```
- a (`JobTemplateSpec`)[https://pkg.go.dev/k8s.io/api/batch/v1beta1#JobTemplateSpec] which defines a `Job` which will be created at the scheduled start times.

But what logic determines what should happen under various situations?

### What happens at a `start` event?
If the schedule indicates that the `ControlledJob` is intended to be running (see the deep dive below on how that is calculated), then we look to see if there are any non-expired `Jobs` owned by the `ControlledJob`. Some definitions:

- 'non-expired' means a `Job` which was started at some point _after_ the most recent scheduled `stop` event
- 'owned by': we set the Kubernetes `OwnerReference` on each created `Job` to the `ControlledJob` which created it so that Kubernetes knows not to automatically garbage collect it, and the `controlled-job-operator` can easily list `Jobs` owned by the `ControlledJob`. This also means tools like ArgoCD which visualise Kubernetes resources will show the `Job` as owned by the `ControlledJob`

If there is a `Job` which is not expired, and is owned by the `ControlledJob`, then no action is taken. **Even if the `Job` itself has failed, completed, or has been unable to even create a `Pod`**. The reason for this is that we can not assume what the user wants to happen when a `Job` fails to start, or completes cleanly or with an error. If we simply restarted a failed `Job` then that might cause issues if the job is non-retryable.

So it's important to remember that when a `ControlledJob` reports as 'running' what it means is that there exists a `Job`. Users can (and should!) monitor both the `Job` itself (and any `Pods` it creates), and the status conditions on the `ControlledJob` which record details about the state of the `Job` and set up alerts as required if a `Job` is not behaving as it should.

### What happens at a `stop` event?
When a `stop` event happens, and `Jobs` owned by the `ControlledJob` with a start time prior to the `stop` event - whatever state they're in and however they got created - are deleted. Deleting a `Job` may not be instantaneous: any `Pod` must be deleted, and that involves a SIGINT signal to the containers and waiting for them to shutdown.

## How a `ControlledJob` is processed - a deep dive

When processing a `ControlledJob` three main things happen:

1. if there are any `Job` objects owned by the `ControlledJob` which were created _before_ the last scheduled `stop` event, then stop them.
1. determine if the `ControlledJob` should potentially be running according to the schedule, and if so ensure that it is (by creating a new Job if necessary)
1. ensure _at most one_ `Job` is potentially running

### 1. Cleanup any expired `Jobs`

The first task is to ensure that if any `Jobs` that are owned by the `ControlledJob` are expired that they are cleaned up.

**What is an expired job?** An expired `Job` is a `Job` that was created earlier than the most recent `stop` event (see the section below for how we determine what the most recent `stop` event was). A `stop` event can be thought of as a 'purge all `Jobs`' action. However the `Jobs` were created, or whatever state they're in, they are immediately deleted.

This ensures we are in a clean state to evaluate if a new `Job` should be created (ie we're not risking treating a `Job` that was running yesterday as the scheduled `Job` for today).

### 2. Ensure the `ControlledJob` is running during its scheduled time

#### Determinine if we should be running
In order to determine whether the `ControlledJob` should be running or not, the code needs to search through all the `events` specified in the spec, and find out if the most recent scheduled event was a `start` action. This works like this:

- Convert all event schedules into CronSchedules
- For each event, search _backwards_ in time from the current time until we find a time which satisifes the crontab specification
- We now have, for each `event` in the spec, a timestamp of the most recent time it happened. We find out of _all_ of those which was the most recent and that's the most recent scheduled event
- If that event was a `start` action, then the `ControlledJob` is considered as `ShouldBeRunning`

##### How daylight savings time changes are handled

Because the logic searches iteratively backwards from the current time, if this involves going over a daylight savings time boundary, different behaviour may be observed:

- if the timezone change involves losing an hour, then the scheduled time may not exist on that day. Be careful not to schedule start times at times of the day when clocks may change in your timezone
- if the timezone change involves gaining an hour (and so a particular wallclock time happens twice in that day), then the second instance of that time will be used for this calculation, not the first

#### Ensure the `ControlledJob` is running

If the `ControlledJob` should be running according to the above logic, we check to see if it has any jobs at present. If it does, **no matter what state that Job is in (running, failed, completed)**, we take no action. If there is _no_ `Job` then one is created according to the `jobTemplate`

Note: if a `--job-admission-webhook-url` is specified on the `controlled-job-operator` and the `ControlledJob` has the `batch.gresearch.co.uk/apply-mutations` annotation, then the generated `Job` is first sent to that `job-admission-webhook-url` to be patched before it is sent to Kubernetes for creation. This allows you to implement on-creation resolution of things like Docker image versions, or add some metadata to the `Job`

### 3. Ensure _at most one_ `Job` is potentially running

It is a fundamental design goal of `ControlledJobs` that **_at most one `Job` should be running at any one time_**, and we take some careful steps to ensure this:

#### Predicatable naming of `Jobs`

We avoid race conditions caused if two instances of the `controlled-job-operator` happened to be running and trying to create a `Job` for the same `ControlledJob` at the same time, by ensuring that the created `Jobs` have a predicatable naming scheme:

`[name of the controlled-job]-[scheduled start time in UNIX time]-[job index]`

`job index` starts at `0` and is incremented each time a new `Job` is required during that run period (eg when the `jobTemplate` changes and we want to recreate the `Job`).

In this way, one client will 'win' and get to create the `Job` and the other will fail (but that's fine) and we never end up with duplicate `Jobs`

#### Delete all but one `Job`

It's possible that something has gone wrong and caused multiple `Jobs` to be running (or potentially running). For example, the user could have used `kubectl` to manually create a `Job`.

Whatever happens, the `controlled-job-operator` will issue delete requests for all but one `Job`. The `Job` to keep is chosen in a stable way (to ensure muiltiple `controlled-job-operators` don't both decide to delete different `Jobs`):

- a `Job` that is not currently being deleted is better to keep than one that is
- a `Job` with a spec which matches the currently defined `jobTemplate` is better than an out of date one
- if there are still conflicts, the one which comes last lexicographically wins

#### Ensure previous jobs have cleanly shutdown before starting new jobs

In cases where `ControlledJobs` are restarted for some reason - typically because the `jobTemplate` has changed and the `specChangePolicy` is set to `recreate` - we want to ensure that the new `Job` does not start running until the old `Job` has completely finished shutting down.

To achieve this, the convention is that _all_ `Jobs` are created in a suspended state to begin with (which means they exist in the cluster, but do not try to start up any `Pods`) and once the `controlled-job-operator` is satisfied that all other `Jobs` have been successfully stopped and removed, it will unsuspend them.

The advantage of this approach is that the new `Job` immediately appears in the cluster, and is visible to the user (so they know their change has been picked up)