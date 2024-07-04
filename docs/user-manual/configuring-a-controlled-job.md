# Configuring a ControlledJob

This page gives information about all the fields available to customize a `ControlledJob`

Full example spec:

```yaml
apiVersion: batch.gresearch.co.uk/v1
kind: ControlledJob
metadata:
  name: my-controlled-job
  annotations:
    batch.gresearch.co.uk/apply-mutations: "true" / "false"
spec:
  events:
  - action: start
    cronSchedule: 0 9 * * MON-FRI
  - action: stop
    schedule:
      timeOfDay: 17:00
      daysOfWeek: MON-FRI
  - action: stop
    schedule:
      timeOfDay: 12:00
      daysOfWeek: SAT,SUN
  timezone:
    name: "GMT"
    offset: 3600

  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: hello
            image: some-image
            
  startingDeadlineSeconds: 1800
  restartStrategy:
    specChangePolicy: recreate
  suspend: false
```

## Annotations

(Optional)

The following annotations can be used to adjust the behaviour of the `ControlledJob`. In the future these may be promoted to full features in the specification itself

- `batch.gresearch.co.uk/apply-mutations`: If the `controlled-job-operator` has been configured with a `--job-admission-webhook-url` then this enables that mutation to occur for `Jobs` created by this `ControlledJob`

## Scheduling

(Optional, but without it your `ControlledJob` will never run)

Scheduling of a `ControlledJob` is managed by specifying a list of `events`. Each event must have:

- an `action`, either `start` or `stop`
- a schedule, either a raw CronTab schedule, or a 'friendly' schedule

A friendly schedule must contain both:
- the time of day it occurs (format `hh:mm`)
- the days of the week it must occur on. This is identical to the way days of the week are specified in a CronTab: either a comma separated list or a range of capitalised three-letter abbreviations, e.g `MON,TUE,FRI` or `WED-SAT`. *Note that day ranges must not cross Saturday to Sunday. That is `SUN-TUE` is fine, and `FRI-SAT` is fine, but `SAT-SUN` is not fine.

## Timezones

(Optional)

By default if no timezone is specified, all scheduled event times will be in UTC. You can specify any valid tzdatabase timezone in `spec.timezone.name` and the timings will be local to that timezone. See https://en.wikipedia.org/wiki/List_of_tz_database_time_zones for a list.

### Timezone offset

In addition, if for some reason you need an additional static offset from a standard timezone, you can add it as `spec.timezone.offset`. The description of this field in the code and in the CRD definition have more details on how this works:

```
	// Additional offset from UTC on top of the specified timezone. If the timezone is normally UTC-2, and
	// OffsetSeconds is +3600 (1h in seconds), then the overall effect will be UTC-1. If the timezone is
	// normally UTC+2 and OffsetSeconds is +3600, then the overall effect will be UTC+3.
	//
	// In practice - if you set this field to 60s on top of a normally UTC-1 timezone, then you end up with
	// a 'UTC-59m' timezone. In that timezone 10:00 UTC == 09:01 UTC-59m. So if you have a scheduled start time
	// of 09:00 in that UTC-59m timezone, your job will be started at 09:59 UTC
```

## Job template

(Required)

This specifies the specification of the `Job` object that will be created at the scheduled times. It is a standard K8s (`JobTemplateSpec`)[https://pkg.go.dev/k8s.io/api/batch/v1beta1#JobTemplateSpec] and so you can refer to the standard K8s documentation for available fields.

Note in particular that `Job` objects in K8s provide options to control what happens when the `Pods` they run complete or fail (for example retry up to a certain number of times), or to run a number of pods in parallel. The `ControlledJob` spec is _deliberately_ unopinionated about how `Pod` failure and so on are handled, as it's expected users will configure their `Jobs` as required. The _only_ job the `controlled-job-operator` has is to ensure a `Job` object exists (in any state: starting up, running, completed, failed, ...) during the scheduled time.

## Other settings

### `startingDeadlineSeconds`
```
  // Optional deadline in seconds for starting the job if it misses scheduled
	// time for any reason. In other words, if a new job is expected to be running, but it's more than
	// startingDeadlineSeconds after the scheduled start time, no job will be created.
	// If not set or set to < 1 this has no effect, and jobs will always be started however long after the start
	// time it is.
	// WARNING!!! Be aware that enabling this setting makes it impossible to restart a controlled job this number of seconds after
	// a scheduled start time. For example if the scheduled start time is 9am and this is set to 3600 (1h), then if you try to restart
	// the controlled job by deleting the current Job any time after 10am, it will have no effect.
```

### `restartPolicy`

This optional block controls how the `ControlledJob` should respond to various triggers which might indicate the current `Job` should be restarted. Currently the only supported trigger is a spec change (`specChangePolicy`), in other words what should happen if the `jobTemplate` for a `ControlledJob` is changed while a `Job` is running:

- `ignore` (default) do nothing. Any existing `Job` will carry on running, and only the next time a new `Job` is created will it get the updated `JobTemplateSpec`
- `recreate` - ff the job is currently running, stop it and wait for it to have completely stopped before starting a new job with the updated spec. Note that if the `Job` is finished (completed or failed), or if it's in the process of being deleted, then no action is taken

### `suspend`

Use this to temporarily disable the `ControlledJob`. If set to `true`, no start actions will be taken on the `ControlledJob` and any `Jobs` will be deleted. In other words it takes immediate effect and stops any running `Jobs`