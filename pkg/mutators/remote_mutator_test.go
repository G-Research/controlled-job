package mutators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	. "github.com/G-Research/controlled-job/pkg/testhelpers"
	"github.com/stretchr/testify/assert"
	jsonpatch "gomodules.xyz/jsonpatch/v2"
	admissionv1 "k8s.io/api/admission/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_RemoteMutatorApply(t *testing.T) {
	originalJob := NewJob("test-job", WithJobAnnotation("foo", "TO_BE_MUTATED"))
	mutatedJob := NewJob("test-job", WithJobAnnotation("foo", "mutated"))
	jobWithNoAnnotations := NewJob("test-job-2")

	client := buildHttpClient()
	url := "https://test/foo/bar"

	testCases := map[string]struct {
		originalJob      *batchv1.Job
		response         func() (*http.Response, error)
		expectedFinalJob *batchv1.Job
		expectedErr      string
	}{
		"successful mutation": {
			originalJob:      originalJob,
			response:         BuildSuccessfulMutationResponse(originalJob, mutatedJob),
			expectedFinalJob: mutatedJob,
			expectedErr:      "",
		},
		"nothing to patch": {
			originalJob:      jobWithNoAnnotations,
			response:         BuildSuccessfulMutationResponse(jobWithNoAnnotations, jobWithNoAnnotations),
			expectedFinalJob: jobWithNoAnnotations,
			expectedErr:      "",
		},
		"failed to send request": {
			originalJob: originalJob,
			response:    func() (*http.Response, error) { return nil, fmt.Errorf("could not connect to service") },
			expectedErr: "failed to send webhook request to https://test/foo/bar: could not connect to service",
		},
		"remote service responds with error code": {
			originalJob: originalJob,
			response:    func() (*http.Response, error) { return makeResponse(400, "bad request"), nil },
			expectedErr: "webhook request returned status code 400: \"bad request\"",
		},
		"remote service responds with invalid body": {
			originalJob: originalJob,
			response:    func() (*http.Response, error) { return makeResponse(200, "{ this is not a valid response}"), nil },
			expectedErr: "failed to read response as AdmissionReview resource: json: cannot unmarshal string into Go value of type v1.AdmissionReview",
		},
		"remote service responds with not allowed": {
			originalJob: originalJob,
			response:    BuildNotAllowedResponse(500, v1.StatusReasonConflict, "could not generate mutation for some reason"),
			expectedErr: "failed to mutate job: 500 Conflict - could not generate mutation for some reason",
		},
		"remote service responds with invalid patch": {
			originalJob: originalJob,
			response:    BuildResponseWithPatch([]byte{1, 2, 3, 4}),
			expectedErr: "failed to decode JSONPatch: invalid character '\\x01' looking for beginning of value",
		},
		"remote service responds with patch that can't be applied": {
			originalJob: originalJob,
			response:    BuildResponseWithPatch([]byte("[{\"op\":\"invalid\",\"path\":\"/metadata/annotations/foo\",\"value\":\"mutated\"}]")),
			expectedErr: "failed to apply JSONPatch: Unexpected kind: invalid",
		},
	}
	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			sut := remoteMutator{
				remoteUrl: url,
				client:    client,
			}

			client.RegiesterResponseForJob(tc.originalJob, tc.response)

			job := tc.originalJob.DeepCopy()

			err := sut.Apply(context.Background(), job)

			if tc.expectedErr != "" {
				assert.NotNil(t, err, "should have generated an error")
				if err != nil {
					assert.Equal(t, tc.expectedErr, err.Error(), "should have generated expected error")
				}
			} else {
				AssertDeepEqualJson(t, tc.expectedFinalJob, job)
			}
		})
	}

}

type mockHttpClient struct {
	// This is a factory method because it would return the same consumed/read Body on identical requests.
	responses map[string]func() (*http.Response, error)
}

func (m *mockHttpClient) Do(req *http.Request) (*http.Response, error) {
	var request *admissionv1.AdmissionReview
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		return makeResponse(http.StatusBadRequest, "failed to decode AdmissionReview"), nil
	}
	var job *batchv1.Job
	if err := json.Unmarshal(request.Request.Object.Raw, &job); err != nil {
		return makeResponse(http.StatusBadRequest, "failed to decode AdmissionReview request as a Job"), nil
	}

	if resp, ok := m.responses[job.Name]; ok {
		return resp()
	}
	return makeResponse(http.StatusNotFound, "no response registered for job"), nil
}

func (m *mockHttpClient) RegiesterResponseForJob(job *batchv1.Job, response func() (*http.Response, error)) {
	m.responses[job.Name] = response
}

func BuildSuccessfulMutationResponse(job, mutatedJob *batchv1.Job) func() (*http.Response, error) {
	jobJson := new(bytes.Buffer)
	json.NewEncoder(jobJson).Encode(job)
	mutatedJobJson := new(bytes.Buffer)
	json.NewEncoder(mutatedJobJson).Encode(mutatedJob)
	patches, _ := jsonpatch.CreatePatch(jobJson.Bytes(), mutatedJobJson.Bytes())
	patch, _ := json.Marshal(patches)
	pt := admissionv1.PatchTypeJSONPatch
	return func() (*http.Response, error) {
		return makeResponse(200, admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				Allowed:   true,
				PatchType: &pt,
				Patch:     patch,
				Result: &v1.Status{
					Code: 200,
				},
			}}), nil
	}
}

func BuildResponseWithPatch(patch []byte) func() (*http.Response, error) {
	pt := admissionv1.PatchTypeJSONPatch
	return func() (*http.Response, error) {
		return makeResponse(200, admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				Allowed:   true,
				PatchType: &pt,
				Patch:     patch,
				Result: &v1.Status{
					Code: 200,
				},
			}}), nil
	}
}
func BuildNotAllowedResponse(code int32, reason v1.StatusReason, message string) func() (*http.Response, error) {
	pt := admissionv1.PatchTypeJSONPatch
	return func() (*http.Response, error) {
		return makeResponse(200, admissionv1.AdmissionReview{
			Response: &admissionv1.AdmissionResponse{
				Allowed:   false,
				PatchType: &pt,
				Result: &v1.Status{
					Code:    code,
					Reason:  reason,
					Message: message,
				},
			}}), nil
	}
}

func buildHttpClient() *mockHttpClient {
	return &mockHttpClient{
		responses: make(map[string]func() (*http.Response, error)),
	}
}

func makeResponse(statusCode int, body interface{}) *http.Response {
	var err error
	var data []byte
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			panic(err)
		}
	}
	// This is a factory method because it would return the same consumed/read Body on identical requests.
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(bytes.NewReader(data)),
	}

}
