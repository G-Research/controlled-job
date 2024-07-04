package metadata

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"

	kbatch "k8s.io/api/batch/v1"
	"k8s.io/api/batch/v1beta1"
)

// CalculateHashFor calculates a SHA256 hash of a k8s job spec in order to facilitate diffing
func CalculateHashFor(spec v1beta1.JobTemplateSpec) (hash string) {
	spec.Spec.Suspend = nil
	// return SHA256 as string of the input Spec
	json, err := json.Marshal(spec)

	if err != nil {
		panic(err)
	}
	sum := sha256.Sum256(json)
	return hex.EncodeToString(sum[:])
}

// IsManuallyScheduledJob returns boolean as to whether job was manually or automatically scheduled
func IsManuallyScheduledJob(job *kbatch.Job) bool {
	for key, annotation := range job.ObjectMeta.Annotations {
		if key != ManualJobAnnotation {
			continue
		}
		isManualJob, err := strconv.ParseBool(annotation)
		if err != nil {
			return false
		}
		return isManualJob
	}
	return false
}

// IsJobPotentiallyRunning determines if it's possible that the given job is running. We need to be paranoid here
// in order to avoid the risk of multiple jobs running at the same time. In the future this could be improved by
// listing the pods associated with the Job. For now this returns true unless the Job has a Complete or Failed condition
func IsJobPotentiallyRunning(job *kbatch.Job) bool {
	return !IsJobCompleted(job)
}

// IsJobCompleted returns true if the job has a Complete or Failed condition with status True
func IsJobCompleted(job *kbatch.Job) bool {
	if JobHasCondition(job, kbatch.JobComplete) || JobHasCondition(job, kbatch.JobFailed) {
		return true
	}
	return false
}

func IsJobBeingDeleted(job *kbatch.Job) bool {
	return job.ObjectMeta.DeletionTimestamp != nil
}

func IsJobSuspended(job *kbatch.Job) bool {
	return job.Spec.Suspend != nil && *job.Spec.Suspend == true
}

func WasJobStoppedByTheUser(job *kbatch.Job) bool {
	suspendReason := job.Annotations[SuspendReason]
	return IsJobSuspended(job) && suspendReason == "user-stop"
}

func JobHasReadyStatus(job *kbatch.Job) bool {
	return job.Status.Ready != nil
}

// IsJobRunning is true if:
// - it's not completed
// - if it advertises a ready count (this is a beta feature), ready is > 1
// - otherwise, if it has any active pods, we have to assume they are running (although this could be a lie)
func IsJobRunning(job *kbatch.Job) bool {
	if IsJobCompleted(job) {
		return false
	}
	if JobHasReadyStatus(job) {
		return (*job.Status.Ready) > 0
	}
	return job.Status.Active > 0
}

func JobHasCondition(job *kbatch.Job, conditionType kbatch.JobConditionType) bool {
	condition := GetJobCondition(job, conditionType)
	if condition == nil {
		return false
	}
	return condition.Status == corev1.ConditionTrue
}

func GetJobCondition(job *kbatch.Job, conditionType kbatch.JobConditionType) *kbatch.JobCondition {
	for _, condition := range job.Status.Conditions {
		if condition.Type == conditionType && condition.Status == corev1.ConditionTrue {
			return &condition
		}
	}
	return nil
}

// We add an annotation to jobs with their scheduled start time
func GetScheduledTime(job *kbatch.Job) (time.Time, error) {
	timeRaw := job.Annotations[ScheduledTimeAnnotation]
	if len(timeRaw) == 0 {
		return time.Time{}, errors.New(fmt.Sprintf("No %s annotation found on job %s", ScheduledTimeAnnotation, job.Name))
	}

	timeParsed, err := time.Parse(time.RFC3339, timeRaw)
	if err != nil {
		return time.Time{}, err
	}
	return timeParsed, nil
}

func GetJobRunId(job *kbatch.Job) (int, error) {
	idxRaw := job.Annotations[JobRunIdAnnotation]
	if len(idxRaw) == 0 {
		return 0, errors.New(fmt.Sprintf("No %s annotation found on job %s", JobRunIdAnnotation, job.Name))
	}

	idxParsed, err := strconv.Atoi(idxRaw)
	if err != nil {
		return 0, err
	}
	return idxParsed, nil
}
