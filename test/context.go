package test

import (
	"context"
	"os"

	"github.com/G-Research/controlled-job/pkg/clientadapter"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

var (
	cfg           *rest.Config
	k8sClient     client.Client
	testEnv       *envtest.Environment
	ctx           context.Context
	cancel        context.CancelFunc
	clientAdapter clientadapter.ControlledJobClient

	enable bool
)

func init() {
	_, enable = os.LookupEnv("ENABLE_INTEGRATION_TESTS")
}
