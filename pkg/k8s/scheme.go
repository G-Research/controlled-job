package k8s

import (
	batchv1 "github.com/G-Research/controlled-job/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(batchv1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func GetScheme() *runtime.Scheme {
	return scheme
}
