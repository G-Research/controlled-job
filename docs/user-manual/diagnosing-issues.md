# Diagnosing issues

If your job doesn't appear to have started on schedule, or it didn't get stopped at the right time, or something else went wrong, you have various options to diagnose the status and history of a `ControlledJob`:

## Status and events on the ControlledJob resource itself

A `ControlledJob` keeps a rich set of status fields, and records actions it takes and errors it encounters as `Events` which appear when doing `kubectl describe`.

The `status` subresource contains:

- A set of standard Kubernetes status conditions. Each records whether the ControlledJob has observed a particular status, such as `JobRunning`, `ShouldBeRunning`, `Error`, `NotRunningUnexpectedly` (ie the `ControlledJob` isn't running, but we expect it to be). These are deliberately numerous and low level, to enable users to build monitoring and alerting to their own requirements. For example you may not care so much if a job keeps running outside of its scheduled time, as long as its always running when it should be, or you may care a lot about the specification of the running job being out of date with what's specified in the template.
- A history of recent actions taken on this `ControlledJob` - such as Jobs created, deleted etc. This is useful to see a timeline of operations to try to work out why a job wasn't running when it should have been
- Details about the currently active `Job` (if any)

## Logs in the operator

These are designed to be accessed by the system administrators to diagnose system-level issues, but consumers may find the logs useful as well to diagnose issues with their `ControlledJob` resources. The logs are fairly verbose but should provide some useful information about what decisions were taken when reconciling a `ControlledJob`, and what `Jobs` were created or deleted.

## Metrics

The `controlled-job-operator` exposes some prometheus metrics on the `metrics` port of its `Service`. These include the standard `controller-runtime` metrics (see https://pkg.go.dev/sigs.k8s.io/controller-runtime/pkg/metrics and https://grafana.com/grafana/dashboards/15920-controller-runtime-controllers-detail/ for a potential Grafana dashboard you can use to visualise them), and a simple `controlledjob_info` metric which records some basic information about each `ControlledJob`