package mutators

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func Test_RegistrationWorked(t *testing.T) {
	EnableRemoteMutator("https://foo/")
	assert.Equal(t, 1, len(registeredMutators))
}
