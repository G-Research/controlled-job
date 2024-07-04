/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package main

import (
	"flag"
	"os"

	"sigs.k8s.io/controller-runtime/pkg/controller"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	"github.com/G-Research/controlled-job/controllers"
	"github.com/G-Research/controlled-job/pkg/clientadapter"
	"github.com/G-Research/controlled-job/pkg/events"
	"github.com/G-Research/controlled-job/pkg/k8s"
	"github.com/G-Research/controlled-job/pkg/mutators"
	"github.com/G-Research/controlled-job/pkg/reconciliation"
	//+kubebuilder:scaffold:imports
)

var (
	setupLog = ctrl.Log.WithName("setup")
)

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var enableAutoRecreateJobsOnSpecChange bool
	var concurrency int
	var remoteWebhookUrl string
	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&enableAutoRecreateJobsOnSpecChange, "enable-auto-recreate-jobs-on-spec-change", false,
		"Enable the new feature to auto-recreate jobs when a spec change is detected")
	flag.IntVar(&concurrency, "concurrency", 1, "Maximum number of controlledJobs to process in parallel")
	flag.StringVar(&remoteWebhookUrl, "job-admission-webhook-url", "", "If set, new jobs will be sent to this URL prior to creation. The remote service is expected to behave like a K8s MutatingAdmissionWebhook and return a patch to be applied")

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	reconciliation.Options.EnableAutoRecreateJobsOnSpecChange = enableAutoRecreateJobsOnSpecChange

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if len(remoteWebhookUrl) > 0 {
		setupLog.Info("enabling remote mutator", "remoteWebhookUrl", remoteWebhookUrl)
		if err := mutators.EnableRemoteMutator(remoteWebhookUrl); err != nil {
			setupLog.Error(err, "unable to enable remote mutator")
			os.Exit(1)
		}
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 k8s.GetScheme(),
		MetricsBindAddress:     metricsAddr,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "4a7b6ad8.gresearch.co.uk",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&controllers.ControlledJobReconciler{
		ControlledJobClient: clientadapter.NewFromClient(mgr.GetClient()),
		Scheme:              mgr.GetScheme(),
		EventHandler:        events.NewHandler(mgr.GetEventRecorderFor("controlled-job-operator")),
	}).SetupWithManager(mgr, controller.Options{
		// Allow multiple reconciles at the same time to prevent one slow reconcile blocking other operations
		MaxConcurrentReconciles: concurrency,
	}); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ControlledJob")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
