package mutators

import (
	"context"
	"net/http"

	"github.com/pkg/errors"
	kbatch "k8s.io/api/batch/v1"
)

var registeredMutators = make(map[string]Mutator)

func EnableRemoteMutator(url string) error {
	return Register(&remoteMutator{
		remoteUrl: url,
		client:    http.DefaultClient,
	})
}

type Mutator interface {
	Name() string
	Apply(ctx context.Context, job *kbatch.Job) error
}

func Register(mutator Mutator) error {
	name := mutator.Name()
	if _, ok := registeredMutators[name]; ok {
		return errors.New("mutator with that name already exists")
	}
	registeredMutators[name] = mutator
	return nil
}

func Unregister(mutator Mutator) error {
	name := mutator.Name()
	if existing, ok := registeredMutators[name]; !ok {
		return errors.New("mutator with that name could not be found")
	} else if existing != mutator {
		return errors.New("mutator does not match")
	}
	delete(registeredMutators, name)
	return nil
}

func Apply(ctx context.Context, job *kbatch.Job) (*kbatch.Job, error) {
	jobCopy := job.DeepCopy()
	for name, mutator := range registeredMutators {
		if err := mutator.Apply(ctx, jobCopy); err != nil {
			return nil, errors.Wrapf(err, "mutator %s failed", name)
		}
	}
	return jobCopy, nil
}
