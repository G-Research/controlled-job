package v1

import (
	"time"

	kbatch "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// SetCondition makes sure the given condition type is set to the given value on the controlledJob
// Will update a condition if one of that type already exists, otherwise will append a new one
// Based on https://github.com/kubernetes/kubernetes/blob/d1a2a134c532109540025c990697a6900c2e62fc/staging/src/k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/helpers.go#L29
func SetCondition(controlledJob *ControlledJob, conditionType ControlledJobConditionType, status metav1.ConditionStatus, reason, message string) {
	newCondition := metav1.Condition{
		Type:               string(conditionType),
		Status:             status,
		LastTransitionTime: metav1.NewTime(time.Now()),
		ObservedGeneration: controlledJob.Generation,
		Reason:             reason,
		Message:            message,
	}

	existingCondition := FindCondition(controlledJob.Status, conditionType)
	if existingCondition == nil {
		controlledJob.Status.Conditions = append(controlledJob.Status.Conditions, newCondition)
		return
	}

	if existingCondition.Status != newCondition.Status || existingCondition.LastTransitionTime.IsZero() {
		existingCondition.LastTransitionTime = newCondition.LastTransitionTime
		existingCondition.ObservedGeneration = newCondition.ObservedGeneration
	}

	existingCondition.Status = newCondition.Status
	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
}

// FindCondition returns the condition you're looking for or nil.
func FindCondition(controlledJobStatus ControlledJobStatus, conditionType ControlledJobConditionType) *metav1.Condition {
	for i := range controlledJobStatus.Conditions {
		if controlledJobStatus.Conditions[i].Type == string(conditionType) {
			return &controlledJobStatus.Conditions[i]
		}
	}

	return nil
}

func SetConditionBasedOnFlag(controlledJob *ControlledJob, conditionType ControlledJobConditionType, flag bool,
	reasonWhenTrue, messageWhenTrue,
	reasonWhenFalse, messageWhenFalse string) {
	if flag {
		SetCondition(controlledJob, conditionType, metav1.ConditionTrue, reasonWhenTrue, messageWhenTrue)
	} else {
		SetCondition(controlledJob, conditionType, metav1.ConditionFalse, reasonWhenFalse, messageWhenFalse)
	}
}

func StatusAsConditionStatus(flag bool) metav1.ConditionStatus {
	if flag {
		return metav1.ConditionTrue
	} else {
		return metav1.ConditionFalse
	}
}

func OptionalStatusAsConditionStatus(flag *bool) metav1.ConditionStatus {
	if flag == nil {
		return metav1.ConditionUnknown
	}
	return StatusAsConditionStatus(*flag)
}

func JobConditionToReason(condition kbatch.JobCondition, prefix string) string {
	if condition.Status == v1.ConditionTrue {
		return prefix + "True"
	} else if condition.Status == v1.ConditionFalse {
		return prefix + "False"
	} else {
		return prefix + "Unknown"
	}
}

func CoerceConditionToBoolen(condition *metav1.Condition) bool {
	if condition == nil {
		return false
	}
	return condition.Status == metav1.ConditionTrue
}

func RemoveCondition(controlledJob *ControlledJob, conditionType ControlledJobConditionType) {
	idx := -1
	for i, condition := range controlledJob.Status.Conditions {
		if condition.Type == string(conditionType) {
			idx = i
			break
		}
	}
	if idx != -1 {
		controlledJob.Status.Conditions = append(controlledJob.Status.Conditions[:idx], controlledJob.Status.Conditions[idx+1:]...)
	}
}
