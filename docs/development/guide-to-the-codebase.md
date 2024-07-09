# A guide to the codebase

This page goes over the files in this repo and gives an overview for what each does.

If you want to explore the codebase for yourself, the best places to start are:

- [main.go](../../main.go) - startup code. Uses the `controller-runtime` library to create and configure a manager to scaffold the operator
- [controlledjob_types.go](../../api/v1/controlledjob_types.go) - defines the `ControlledJob` resource. The specification, status and related types
- [reconcile.go](../../pkg/reconciliation/reconcile.go) - this is the main reconcile function. Its given the details of a `ControlledJob` which might need to be reconciled, and it: loads the current state of the `ControlledJob` and any child `Jobs`, makes a decision on what action to take (if any), takes any required action (create new Jobs, delete old Jobs etc), records the current status of the resource
- [integration tests](../../pkg/reconciliation/test/) - this set of files defines a comprehensive set of tests of the reconcile logic.

### `api`

This folder defines the `ControlledJob` resource and the `GroupVersion` it lives in:

```
	// GroupVersion is group version used to register these objects
	GroupVersion = schema.GroupVersion{Group: "batch.gresearch.co.uk", Version: "v1"}
```

### `cli`

We provide a CLI for users to interact with `ControlledJobs` in a simpler way than going directly via `kubectl`. At the moment this just provides a way to template out a `Job` for a given `ControlledJob`. See [Manually created jobs](../user-manual/manually-created-jobs.md)

### `config`

This is a set of Kustomize files, mostly generated automatically by the `kubebuilder` tool or by automatic code generation. For example the CRD definition for `ControlledJobs` is automatically generated from the types under `api/v1` when you run `make manifests`

### `controllers`

This is another `kubebuilder` generated folder. It contains the `controlledjob_controller` which gets registered in the `controller-runtime` manager to handle `ControlledJob` reconcile requests. All of the actual reconcile logic lives in `pkg/reconciliation` though.

### `deploy`

Contains resources to deploy the ControlledJob system to a cluster. At the moment this is just a helm chart. You can regenerate parts of it by running `make generate-chart`

### `docs`

Documentation

### `hack`

Contains the template of the license header to be added to all auto-generated source files

### `pkg`

This is where the bulk of the code lives

#### `clientadapter`

To simplify our interactions with the Kubernetes API and client code, this package provides an abstraction interface for the operations we need to perform (create job, delete job etc)

#### `events`

We care a lot about recording as much information about the operation of the `ControlledJob` as possible. This package contains things like code to emit regular Kubernetes events (that show up in `kubectl describe controlledjob`), as well as records of actions taken which are added to the `ControlledJob`'s status.

#### `job`

Provides the logic to build a new `Job` object based on a `ControlledJob` `jobTemplate`

#### `k8s`

Tiny package which holds the K8s `scheme` which we use to register and refer to `ControlledJob` objects in interactions with the API

#### `metadata`

Defines the annotations that get added to created `Jobs`, as well as some helper methods which determine various properties of a `Job` (is it suspended? Completed?)

#### `metrics`

Package to register and record any metrics exposed by the `ControlledJob` operator for Prometheus.

#### `mutators`

We provide the ability for the definition of a new `Job` to be mutated just before it is sent to Kubernetes for creation. This could be used to add common metadata to all `Jobs`, or dynamically lookup the correct Docker image to launch.

#### `reconciliation`

This is where the core logic of the system is defined, as well as a suite of integration tests to test different scenarios and edge cases

#### `schedule`

This package encapsulates the logic to handle scheduling, timezones and cron formats.

#### `testhelpers`

Utilities to make testing easier, particularly creating dummy resource definitions for tests

### `test`

The scaffolding here is provided by the `kubebuilder` tool and provides a testbed where a real K8s API is spun up in order to test the system end-to-end. We don't make much use of this as most of the integration testing is done in `pkg/reconciliation/test`