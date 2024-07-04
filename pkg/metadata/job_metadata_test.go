package metadata

import (
	"encoding/json"
	"testing"
	"time"

	. "github.com/G-Research/controlled-job/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	batch "k8s.io/api/batch/v1"
	kbatch "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_CalculateHashFor(t *testing.T) {
	testCases := map[string]struct {
		input        v1beta1.JobTemplateSpec
		expectedHash string
	}{
		"empty_input": {
			expectedHash: "d51250825a75d036064852bab012096e6dc1c6299d0d964e2fe9a8e4a2ec4aeb",
			input:        v1beta1.JobTemplateSpec{},
		},
		"simple_input": {
			expectedHash: "c5f7f2e0bcebed62047d5eb3d016b1e6868c445b382bbe4714396efc34cec555",
			input: v1beta1.JobTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
					},
				},
			},
		},
		"less_simple_input": {
			expectedHash: "739248d64d22ee14c47cf7a0cb9558e7d1e8d6373c1b112fca95008bcc8fa685",
			input: v1beta1.JobTemplateSpec{
				ObjectMeta: v1.ObjectMeta{
					Annotations: map[string]string{
						"foo": "bar",
						"ram": "you",
					},
					Name:              "boggle",
					Namespace:         "toggle-boggle",
					UID:               "b-l-a_r_g_l_e",
					CreationTimestamp: v1.Date(1999, time.December, 31, 11, 59, 59, 59, time.FixedZone("UTC-8", -8*60*60)),
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			// Log out string being parsed on failure
			jsonString, _ := json.Marshal(tc.input)
			t.Log(string(jsonString))
			// Actual test
			actual := CalculateHashFor(tc.input)
			assert.Equal(t, tc.expectedHash, actual)
		})
	}

}

func Test_IgnoresSuspendFlagInComputingHash(t *testing.T) {
	suspend := true
	withFlag := v1beta1.JobTemplateSpec{
		Spec: batch.JobSpec{
			Suspend: &suspend,
		},
	}
	withoutFlag := v1beta1.JobTemplateSpec{
		Spec: batch.JobSpec{
			Suspend: nil,
		},
	}

	assert.Equal(t, CalculateHashFor(withoutFlag), CalculateHashFor(withFlag), "should have same hash with or without the suspend flag")
	assert.Equal(t, &suspend, withFlag.Spec.Suspend, "should not have modified the input")
}

func Test_JobRunningStates(t *testing.T) {
	// Test the various helpers which try to determine whether the Job is running or not etc

	t.Run("IsManuallyScheduledJob", func(t *testing.T) {
		testCases := map[string]struct {
			job      *kbatch.Job
			expected bool
		}{
			"No manual-job annotation": {
				NewJob("job"),
				false,
			},
			"Annotation != true": {
				NewJob("job", WithJobAnnotation(ManualJobAnnotation, "false")),
				false,
			},
			"Annotation == true": {
				NewJob("job", WithJobAnnotation(ManualJobAnnotation, "true")),
				true,
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				actual := IsManuallyScheduledJob(tc.job)
				assert.Equal(t, tc.expected, actual)
			})
		}
	})

	t.Run("IsJobPotentiallyRunning / IsJobCompleted", func(t *testing.T) {
		testCases := map[string]struct {
			job             *kbatch.Job
			expectedRunning bool
		}{
			"No conditions": {
				NewJob("job"),
				true,
			},

			"JobComplete is Unknown": {
				NewJob("job", WithCondition((kbatch.JobCondition{
					Type:   kbatch.JobComplete,
					Status: corev1.ConditionUnknown,
				}))),
				true,
			},
			"JobComplete is False": {
				NewJob("job", WithCondition((kbatch.JobCondition{
					Type:   kbatch.JobComplete,
					Status: corev1.ConditionFalse,
				}))),
				true,
			},
			"JobComplete is True": {
				NewJob("job", WithCondition((kbatch.JobCondition{
					Type:   kbatch.JobComplete,
					Status: corev1.ConditionTrue,
				}))),
				false,
			},

			"JobFailed is Unknown": {
				NewJob("job", WithCondition((kbatch.JobCondition{
					Type:   kbatch.JobFailed,
					Status: corev1.ConditionUnknown,
				}))),
				true,
			},
			"JobFailed is False": {
				NewJob("job", WithCondition((kbatch.JobCondition{
					Type:   kbatch.JobFailed,
					Status: corev1.ConditionFalse,
				}))),
				true,
			},
			"JobFailed is True": {
				NewJob("job", WithCondition((kbatch.JobCondition{
					Type:   kbatch.JobFailed,
					Status: corev1.ConditionTrue,
				}))),
				false,
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				actual := IsJobPotentiallyRunning(tc.job)
				actualCompleted := IsJobCompleted(tc.job)
				assert.Equal(t, tc.expectedRunning, actual)
				assert.Equal(t, !tc.expectedRunning, actualCompleted)
			})
		}
	})

	t.Run("IsJobRunning", func(t *testing.T) {
		testCases := map[string]struct {
			job             *kbatch.Job
			expectedRunning bool
		}{
			"No conditions": {
				NewJob("job"),
				false,
			},
			"Job is complete": {
				NewJob("job", WithCondition((kbatch.JobCondition{
					Type:   kbatch.JobComplete,
					Status: corev1.ConditionTrue,
				}))),
				false,
			},
			"Job is not complete, has ready count == 0": {
				NewJob("job", WithCondition((kbatch.JobCondition{
					Type:   kbatch.JobComplete,
					Status: corev1.ConditionTrue,
				})), WithReadyCount(0)),
				false,
			},
			"Job is not complete, has ready count == 1": {
				NewJob("job", WithCondition((kbatch.JobCondition{
					Type:   kbatch.JobComplete,
					Status: corev1.ConditionTrue,
				})), WithReadyCount(1)),
				false,
			},
			"Job is not complete, has no ready count and active count == 0": {
				NewJob("job", WithCondition((kbatch.JobCondition{
					Type:   kbatch.JobComplete,
					Status: corev1.ConditionTrue,
				})), WithActiveCount(0)),
				false,
			},
			"Job is not complete, has no ready count and active count == 1": {
				NewJob("job", WithCondition((kbatch.JobCondition{
					Type:   kbatch.JobComplete,
					Status: corev1.ConditionTrue,
				})), WithActiveCount(1)),
				false,
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				actual := IsJobRunning(tc.job)
				assert.Equal(t, tc.expectedRunning, actual)
			})
		}
	})

	t.Run("IsJobBeingDeleted", func(t *testing.T) {
		now := v1.Now()
		testCases := map[string]struct {
			job      *kbatch.Job
			expected bool
		}{
			"No deletion timestamp": {
				NewJob("job"),
				false,
			},
			"Non-nil deletion timestamp": {
				NewJob("job", WithJobDeletionTimestamp(&now)),
				true,
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				actual := IsJobBeingDeleted(tc.job)
				assert.Equal(t, tc.expected, actual)
			})
		}
	})
	t.Run("IsJobSuspended", func(t *testing.T) {
		yes := true
		no := false
		testCases := map[string]struct {
			job      *kbatch.Job
			expected bool
		}{
			"No suspend flag": {
				NewJob("job", WithSuspendFlag(nil)),
				false,
			},
			"suspend == false": {
				NewJob("job", WithSuspendFlag(&no)),
				false,
			},
			"suspend == true": {
				NewJob("job", WithSuspendFlag(&yes)),
				true,
			},
		}
		for name, tc := range testCases {
			t.Run(name, func(t *testing.T) {
				actual := IsJobSuspended(tc.job)
				assert.Equal(t, tc.expected, actual)
			})
		}
	})

}
