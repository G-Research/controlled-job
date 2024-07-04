package mutators

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/G-Research/controlled-job/pkg/mutators/utils"
	admissionv1 "k8s.io/api/admission/v1"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/log"

	jsonpatch "github.com/evanphx/json-patch"
	"k8s.io/apimachinery/pkg/util/uuid"
)

type remoteMutator struct {
	remoteUrl string
	client    utils.UrlGetter
}

var _ Mutator = &remoteMutator{}

// Apply implements mutators.Mutator.
func (r *remoteMutator) Apply(ctx context.Context, job *batchv1.Job) error {
	request := buildAdmissionReview(job)
	response, err := makeWebhookRequest(ctx, r.client, r.remoteUrl, request)
	if err != nil {
		return err
	}

	if !response.Allowed {
		return fmt.Errorf("failed to mutate job: %d %s - %s", response.Result.Code, response.Result.Reason, response.Result.Message)
	}

	if len(response.Patch) == 0 {
		// Nothing to patch
		return nil
	}

	patchObj, err := jsonpatch.DecodePatch(response.Patch)
	if err != nil {
		return fmt.Errorf("failed to decode JSONPatch: %w", err)
	}

	jobJS := new(bytes.Buffer)
	json.NewEncoder(jobJS).Encode(job)

	res, err := patchObj.Apply(jobJS.Bytes())
	if err != nil {
		return fmt.Errorf("failed to apply JSONPatch: %w", err)
	}

	if err = json.Unmarshal(res, &job); err != nil {
		return fmt.Errorf("failed to unmarshal result back to a Job object: %w", err)
	}

	return nil
}

// Name implements mutators.Mutator.
func (r *remoteMutator) Name() string {
	return "remote"
}

func buildAdmissionReview(job *batchv1.Job) admissionv1.AdmissionReview {
	return admissionv1.AdmissionReview{
		Request: &admissionv1.AdmissionRequest{
			UID:       uuid.NewUUID(),
			Kind:      v1.GroupVersionKind{Group: "batch", Version: "v1", Kind: "Job"},
			Resource:  v1.GroupVersionResource{Group: "batch", Version: "v1", Resource: "jobs"},
			Name:      job.Name,
			Namespace: job.Namespace,
			Operation: admissionv1.Create,
			Object: runtime.RawExtension{
				Object: job,
			},
		}}
}

func makeWebhookRequest(ctx context.Context, client utils.UrlGetter, url string, request admissionv1.AdmissionReview) (*admissionv1.AdmissionResponse, error) {
	log := log.FromContext(ctx).
		WithValues(
			"url", url,
			"requestID", request.Request.UID,
		)

	payloadBuf := new(bytes.Buffer)
	json.NewEncoder(payloadBuf).Encode(request)

	req, err := http.NewRequest(http.MethodPost, url, payloadBuf)
	if err != nil {
		return nil, fmt.Errorf("failed to build webhook request to %s: %w", url, err)
	}
	req.Header.Add("Content-Type", "application/json")
	log.Info("sending request...")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send webhook request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	log.Info("response received", "statusCode", resp.StatusCode)

	if resp.StatusCode >= 300 {
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("webhook request returned status code %d, and failed to read body of response: %w", resp.StatusCode, err)
		}
		return nil, fmt.Errorf("webhook request returned status code %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var response *admissionv1.AdmissionReview
	if err = json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to read response as AdmissionReview resource: %w", err)
	}
	if response.Response != nil {
		var statusCode int32 = 0
		if response.Response.Result != nil {
			statusCode = response.Response.Result.Code
		}
		log.Info("parsed response", "allowed", response.Response.Allowed, "patchLength", len(response.Response.Patch), "resultStatusCode", statusCode)
	}
	return response.Response, nil
}
