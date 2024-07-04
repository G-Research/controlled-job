# Introduction to developing on the ControlledJob project

The code in this project uses common Kubernetes libraries and conventions, to reduce the amount of boilerplate code, and make it easy for people familiar with working on K8s custom resources to dive straight in.

## General guidance on developing a Kubernetes controller

Writing a custom resource type, and a controller to manage it, is a very common and well documented process in the Kubernetes world, so you'll find lots of advice around the internet.

The project was bootstrapped using the [`kubebuilder`](https://github.com/kubernetes-sigs/kubebuilder) tool, which provides a CLI to build the scaffolding for a new K8s API / resource / controller, and the [kubebuilder book](https://book.kubebuilder.io/) is a great resource for getting started.

The [`controller-runtime`][https://github.com/kubernetes-sigs/controller-runtime] library does a lot of the heavy lifting in setting up and registering a controller with the K8s API, watching for changes to the custom resource (and any dependencies) and dealing with errors, metrics, etc.

The general pattern of implementing a controller for a new CRD is:
- Create a `controller-runtime` manager, and tell it about the CRD you are managing, and point it towards a...
- Controller type, which wil define a `Reconcile()` function. That function will be called any time the `controller-runtime` thinks there has been a change to one of your CRDs or a resource that they own. The job of the reconcile function is to ensure that the cluster state matches the desired state the user has defined in the CRD spec.

## Where to start

The best places to start to understand the code are:

- The `ControlledJob` type definitions ([source code](api/v1/controlledjob_types.go)). This defines the fields and types which make up the `ControlledJob` CRD, as well as helper types like status conditions. Read it to get a feel for the shape of the specification and what features are available
- The `ControlledJob` controller is the code that the `controller-runtime` calls into to process changes to `ControlledJobs` ([source code](controllers/controlledjob_controller.go)). That is then the entry point into the main logic of this repo, which lives under [pkg/reconciliation/reconcile.go](pkg/reconciliation/reconcile.go).

## Local development

There is a [MAKEFILE](MAKEFILE) in the root of this repo which provides shortcuts to some common operations:

- Build: `make build`
- Test: `make unit-test` for just unit tests, and `make test` which runs some integration tests using an in-memory fake Kubernetes API (we don't do much testing via this method though - most of the real integration tests are under [pkg/reconciliation/test](pkg/reconciliation/test))
- Regenerate code: `make manifests` (regenerates the CRD definition and others), `make generate` (regenerates any auto-generated go code)