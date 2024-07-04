package job

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/G-Research/controlled-job/pkg/metadata"
	"github.com/G-Research/controlled-job/pkg/testhelpers"
	. "github.com/G-Research/controlled-job/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	kbatch "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

var scheme = runtime.NewScheme()

func init() {
	utilruntime.Must(batch.AddToScheme(scheme))
	utilruntime.Must(kbatch.AddToScheme(scheme))
}

func Test_BuildForControlledJob(t *testing.T) {

	controlledJobName := "my-controlled-job"
	uid := types.UID("abcdef-12345")
	scheduledTime := time.Date(2022, 1, 14, 15, 9, 0, 0, time.UTC)

	jobTemplateWithAnnotationsAndLabels := NewJobTemplate(
		WithJobTemplateAnnotations(map[string]string{"foo": "bar"}),
		WithJobTemplateLabels(map[string]string{"a": "b", "c": "d"}),
	)

	t.Log(fmt.Printf("+%v", jobTemplateWithAnnotationsAndLabels))

	jobTemplateWithSpec := NewJobTemplate(
		WithPodTemplate(
			*NewPodTemplate(WithPodSpec(DefaultPodSpec())),
		),
	)

	// 1642172940 = 2022-1-14 15:09:00 in unix timestamp
	expectedJobName := "my-controlled-job-1642172940-0"

	testCases := map[string]struct {
		controlledJob *batch.ControlledJob
		expectedJob   *kbatch.Job
		expectedError string
	}{
		"Uses correct name format and adds correct metadata": {
			controlledJob: NewControlledJob(controlledJobName,
				WithUID(uid)),
			expectedJob: NewJob(expectedJobName,
				// This adds the annotations and owner references which
				// link the job back to the controlled job
				metadata.WithControlledJobMetadata(controlledJobName, uid, scheduledTime, 0, v1beta1.JobTemplateSpec{})),
		},
		"Copies annotations and labels from ControlledJob JobTemplate": {
			controlledJob: NewControlledJob(controlledJobName,
				WithUID(uid),
				WithJobTemplate(*jobTemplateWithAnnotationsAndLabels),
			),
			expectedJob: NewJob(expectedJobName,
				// Note that the annotations and labels from the job template are 'promoted' to the
				// annotations and labels of the job itself
				WithJobAnnotations(jobTemplateWithAnnotationsAndLabels.Annotations),
				WithJobLabels(jobTemplateWithAnnotationsAndLabels.Labels),
				metadata.WithControlledJobMetadata(controlledJobName, uid, scheduledTime, 0, *jobTemplateWithAnnotationsAndLabels)),
		},
		"Copies pod template from ControlledJob JobTemplate": {
			controlledJob: NewControlledJob(controlledJobName,
				WithUID(uid),
				WithJobTemplate(*jobTemplateWithSpec),
			),
			expectedJob: NewJob(expectedJobName,
				WithTemplate(jobTemplateWithSpec),
				metadata.WithControlledJobMetadata(controlledJobName, uid, scheduledTime, 0, *jobTemplateWithSpec)),
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			actualJob, actualErr := BuildForControlledJob(context.Background(), tc.controlledJob, scheduledTime, 0, false, false)
			// Log out useful info on failure
			t.Log(string(metadata.CalculateHashFor(tc.controlledJob.Spec.JobTemplate)))
			jsonString, _ := json.Marshal(tc.controlledJob)
			t.Log(string(jsonString))
			jsonString2, _ := json.Marshal(actualJob)
			t.Log(string(jsonString2))
			if tc.expectedError != "" {
				assert.NotNil(t, actualErr, "should have returned an error")
				if actualErr != nil {
					assert.Equal(t, tc.expectedError, actualErr.Error())
				}
			}

			testhelpers.AssertDeepEqualJson(t, tc.expectedJob, actualJob)
		})
	}
}
