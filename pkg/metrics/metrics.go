package metrics

import (
	"fmt"

	batch "github.com/G-Research/controlled-job/api/v1"
	"github.com/prometheus/client_golang/prometheus"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
)

// Set of custom metrics for the ControlledJob operator
// Partially based on the conventions in https://github.com/kubernetes/kube-state-metrics/blob/master/docs/cronjob-metrics.md

var (
	ControlledJobInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "controlledjob_info",
			Help: "Information about ControlledJobs",
		},
		[]string{"namespace", "controlledjob", "timezone"},
	)
)

func init() {
	metrics.Registry.MustRegister(
		ControlledJobInfo,
	)
}

func ControlledJobInfoLabelValuesFor(controlledJob *batch.ControlledJob) []string {
	timezoneDesc := fmt.Sprintf("%s+%d", controlledJob.Spec.Timezone.Name, controlledJob.Spec.Timezone.OffsetSeconds)
	return []string{controlledJob.Namespace, controlledJob.Name, timezoneDesc}
}
