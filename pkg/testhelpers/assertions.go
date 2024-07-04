package testhelpers

import (
	"encoding/json"
	"github.com/stretchr/testify/assert"
)

func AssertDeepEqualJson(t assert.TestingT, expected interface{}, actual interface{}, msgAndArgs ...interface{}) {
	// Convert to JSON to make it easier to work out what's not matching
	expectedJson := ""
	actualJson := ""
	if expected != nil {
		b, _ := json.MarshalIndent(expected, "", "  ")
		expectedJson = string(b)
	}

	if actual != nil {
		b, _ := json.MarshalIndent(actual, "", "  ")
		actualJson = string(b)
	}

	assert.Equal(t, expectedJson, actualJson, msgAndArgs)
}

func AssertSameError(t assert.TestingT, expected error, actual error, msgAndArgs ...interface{}) {
	if expected == nil {
		assert.Nil(t, actual, msgAndArgs)
	} else {
		assert.NotNil(t, actual, msgAndArgs)
		if actual != nil {
			assert.Equal(t, expected.Error(), actual.Error(), msgAndArgs)
		}
	}
}
