package metadata

import (
	"fmt"

	batch "github.com/G-Research/controlled-job/api/v1"
)

var (
	JobOwnerKey                     = ".metadata.controller"
	ApiGVStr                        = batch.GroupVersion.String()
	ScheduledTimeAnnotation         = fmt.Sprintf("%s/scheduled-at", batch.GroupVersion.Group)
	JobRunIdAnnotation              = fmt.Sprintf("%s/job-run-id", batch.GroupVersion.Group)
	ControlledJobLabel              = fmt.Sprintf("%s/controlled-job", batch.GroupVersion.Group)
	ManualJobAnnotation             = fmt.Sprintf("%s/is-manually-scheduled", batch.GroupVersion.Group)
	TemplateHashAnnotation          = fmt.Sprintf("%s/job-template-hash", batch.GroupVersion.Group)
	SuspendReason                   = fmt.Sprintf("%s/suspend-reason", batch.GroupVersion.Group)
	ApplyMutationsAnnotation        = fmt.Sprintf("%s/apply-mutations", batch.GroupVersion.Group)
	TimeZoneAnnotation              = fmt.Sprintf("%s/timezone", batch.GroupVersion.Group)
	TimeZoneOffsetSecondsAnnotation = fmt.Sprintf("%s/timezone-offset-seconds", batch.GroupVersion.Group)
)
