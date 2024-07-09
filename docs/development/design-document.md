# Design document

This page sets out the design decisions and requirements that went into the development of the ControlledJob system.

## The problem statement

### Scheduling of workloads

At G-Research we run a lot of workloads which need to be constantly running during particular times of the day, but must be stopped outside of those times (in order to free up unused resources, make sure external resources are not accessed outside of allowed hours, or to ensure each day a new instance is launched with a new set of input data). This lies somewhere between two existing Kubernetes concepts:

- A `CronJob` specifies a workload to be run on a particular schedule, but it's tailored to batch processing: 'run this `Job` until it completes, however long that takes'. This doesn't meet our requirements of constantly running during our operational hours (the `Job` could complete early), or the requirement to stop running outside of those hours: a `CronJob` provides no way to halt a running `Job` if it runs too long.
- There are various workload management concepts in Kubernetes which specify a workload that should _always_ be running. For example `Deployments` and `StatefulSets`. These satisfy the requirement to continuously run when enabled, but we need a way to schedule the workload to start at a given time and stop at a later time.

`ControlledJobs` sit between these two: the scheduling of a `CronJob` with the implied continuous running of a `Deployment`.

### Manual control

As well as automatic scheduling of these workloads, there are times when manual control over them is required:

- An operator may want to start a workload early one day, in order to test out a new feature out of hours when the impact of any issues is reduced
- The ability to suspend a running workload for operational reasons
- Some workloads depend on the output of batch data preparation jobs, which can take a varying amount of time to run, and so there needs to be a way to trigger the _starting_ of these workloads at an arbitrary time, but still have the _stop_ time respected
- Restarting a failed workload during the day - either to clear some state (out of memory for example), or to pick up a hotfix. See below for more on failure handling

### Failure handling

Many of the workloads that G-Research run have strict requirements over failure handling. We may not always want failing workloads to automatically restart, if on a restart they will repeatedly send the same request to external services if that incurrs a fee. We rely on monitoring and alerting for human operators to be paged when workflows fail so they can take appropriate action. In other cases, teams prefer an automatic retry of failed workloads.

Therefore the system must be agnostic to how failures are handled and allow the user control over it.

### Exclusive running

Our users have a strict requirement that only one instance of a workload may run at any one time, and so any solution needed to respect that requirement.

## The design

A lot of the choices in the design below come out of the design philosophy of Kubernetes itself. We cannot rely on having an accurate record of the history of operations on the resources we care about. For example it's possible a `Job` is created and deleted between reconcile operations on a `ControlledJob` (if the operator was down during that period say) and so we can't rely on accurately knowing how many `Jobs` have so far been scheduled for the `ControlledJob` on that day. We must base decisions purely on the currently observed state of the cluster.

This has implications in how we handle the processing of `ControlledJobs`. For example, when there are no `Jobs` for the `ControlledJob`, we can't distinguish whether that's because one hasn't yet been created today at all, or that one was created, ran, failed and got deleted.

### The choice to use `Jobs`

The `CronJob` resource was the jumping off point for this design. It creates `Jobs` on a schedule, and a `Job` already has scope to spin up a `Pod` which runs until completion, and to specify a failure and retry policy. Because of this `Jobs` were chosen as the mechanism to actually run the specified workloads.

This means the role of the `ControlledJob` operator is to manage the `Jobs` it owns: ensure that the correct `Job` is created at the right time, and that `Jobs` are deleted when needed.

### What invariants does the operator satisfy

The design of Kubernetes itself is built on desired state and eventual consistency. An operator responsible for a resource type is expected to be able to compare desired state in the resource's specification and reconcile that to reailty by performing various actions. One way to view this is that a reconcile operation should maintain a set of invariants on the system.

In the case of a `ControlledJob` they are as follows. You can see this in action in the [MakeDecision() function](../../pkg/reconciliation/decision.go):

#### 1. When the `ControlledJob` is set to suspended, all `Jobs` are deleted and no further action is taken

#### 2. When the `ControlledJob` is scheduled to be running, a single suitable `Job` object exists

This invariant implements the start event of a schedule

One key design decision is that it's only the _presence_ of a `Job` which is required to satisfy the scheduler - it does not care if the `Job` is stuck in a starting state, if it has failed, of if it has exited cleanly. As noted above, a `Job` already has the ability to specify how to handle Job completion, pre-emption (when the node a `Pod` is running on is shutdown for example) or failure (a container exiting with a non-0 exit code), and it would be counterproductive to try to duplicate or override that logic at the `ControlledJob` level. In addition, `Jobs` and `Pods` already expose events and status conditions to report various states and so for reporting and monitoring purposes it's better for users to go to the source of truth - the `Jobs` and `Pods` than for the `ControlledJob` to try to editorialise that data.

This design choice - to only care about the _presence_ of a `Job`, not its status - provides a clean separation between the scheduling of the workloads, and the monitoring/alerting of them.

In this case a 'suitable' `Job` means a `Job` which:

- Has the correct metadata to identify it as a `Job` owned by the `ControlledJob`. See [Job metadata](../user-manual/job-metadata.md)
- Has a scheduled start time in the current run period (ie it wasn't started prior to the last scheduled stop time)

In addition, if the `ControlledJob` has a `spec.restartStrategy.specChangePolicy` of `Recreate`, the `Job` must have a specification which matches the template spec on the `ControlledJob`. If it doesn't then the running `Job` is enqueued for deletion, and a new `Job` (with the new specification) is enqueued for running.

Any `Job` found which does not match those requirements is enqueued for deletion.

If _no_ suitable `Job` is found, then a new `Job` is enqueued for running.

The requirement for there to be a _single_ suitable `Job` means that if multiple suitable `Jobs` are found, all but one are enqueued for deletion, using a stable sort algorithm to ensure a predictable `Job` is chosen as the survivor. In the code this is referred to as the `chosenJob`

#### 3. Any `Job` with a scheduled start time prior to the last scheduled stop time is deleted

This invariant implements the stop event of a schedule. One of the pieces of metadata on a created `Job` is its scheduled start time. If this is determined to be earlier than the most recent `stop` event in the schedule then the `Job` is enqueued for deletion.

Note, this is subtly different to saying that when the `ControlledJob` is not scheduled to be running we delete all `Jobs`. In many cases this would have the same effect, but simply deleting all `Jobs` when the schedule says to not be running would not allow `Jobs` to be manually started outside of the schedule, which is one of the design goals.

This version of the logic makes the `stop` events act like a garbage collector: at those times any old `Jobs` (in any state) are set for deletion.

#### 4. If there is a `Job` waiting to start and there are no other `Jobs` which could potentially be running, unsuspend the `Job`

This helps to ensure the exclusive running of a `ControlledJob`. Newly scheduled `Jobs` are created with the `suspend` flag set. This means they appear as resources in Kubernetes, but the `JobController` will not schedule any `Pods` for them. The `controlled-job-operator` reconcile function will only _unsuspend_ new `Jobs` once it is satisfied there can be no other `Jobs` still running.

## Handling manually scheduled `Jobs`

Jobs manually scheduled by the user introduce some additional complexity. The main thing we need to avoid is contradicting the user's intention. If they have deliberately created a new `Job` we should not immediately delete it. Similarly if they have deliberately stopped a `Job` we shouldn't immediately recreate it.

We make use of annotations on created `Jobs` to record some of this state and help the `ControlledJob` reconcile function know how to handle manually created jobs. The `batch.gresearch.co.uk/is-manually-scheduled` annotation should be set to `"true"` on any manually created `Job`. 

If the user wants to stop a running `Job` (before potentially restarting it later in the day), that's not possible by simply deleting the `Job`: the `controlled-job-operator` will simply see that no `Job` exists when there should be one and create a new one. Instead, the user must set the `Job` to be suspended by setting its `suspend` flag. In addition the `batch.gresearch.co.uk/suspend-reason` annotation should be set so that the `controlled-job-operator` knows not to unsuspend the `Job` as it normally would when starting up a new `Job`.
