package mutation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/webhooks/utils/defaults"
	webhooktelemetry "github.com/krateoplatformops/core-provider/internal/telemetry/webhooks"
	crdtools "github.com/krateoplatformops/core-provider/internal/tools/crd"
	"gomodules.xyz/jsonpatch/v2"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func NewWebhookHandler(cli client.Reader, metrics ...*webhooktelemetry.Metrics) *webhook.Admission {
	var recorder *webhooktelemetry.Metrics
	if len(metrics) > 0 {
		recorder = metrics[0]
	}

	return &webhook.Admission{
		Handler: admission.HandlerFunc(func(ctx context.Context, req webhook.AdmissionRequest) webhook.AdmissionResponse {
			started := time.Now()
			operation := string(req.Operation)
			if operation == "" {
				operation = "unknown"
			}
			success := false
			defer func() {
				if recorder != nil {
					recorder.RecordRequest(ctx, "mutating", operation, time.Since(started), success)
				}
			}()

			unstructuredObj := &unstructured.Unstructured{}
			if err := json.Unmarshal(req.Object.Raw, unstructuredObj); err != nil {
				return webhook.Errored(http.StatusBadRequest, err)
			}
			// Get CRD from the request
			crd, err := crdtools.Get(ctx, cli, schema.GroupResource{
				Group:    req.Kind.Group,
				Resource: req.Resource.Resource,
			})
			if err != nil {
				return webhook.Errored(http.StatusBadRequest, err)
			}
			if crd == nil {
				return webhook.Errored(http.StatusBadRequest, fmt.Errorf("CRD not found"))
			}

			bReqObj := req.Object.Raw

			modObj, _, err := unstructured.UnstructuredJSONScheme.Decode(req.Object.Raw, nil, nil)
			if err != nil {
				return webhook.Errored(http.StatusBadRequest, err)
			}
			err = defaults.PopulateDefaultsFromCRD(crd, &modObj)
			if err != nil {
				return webhook.Errored(http.StatusBadRequest, err)
			}

			bMod, err := json.Marshal(modObj)
			if err != nil {
				return webhook.Errored(http.StatusBadRequest, err)
			}

			patch, err := jsonpatch.CreatePatch(bReqObj, bMod)
			if err != nil {
				return webhook.Errored(http.StatusBadRequest, err)
			}
			if len(patch) == 0 {
				patch = []jsonpatch.JsonPatchOperation{}
			}

			if req.Operation == v1.Create {
				labels := unstructuredObj.GetLabels()
				if len(labels) == 0 {
					patch = append(patch,
						webhook.JSONPatchOp{Operation: "add", Path: "/metadata/labels", Value: map[string]string{}},
						webhook.JSONPatchOp{Operation: "add", Path: "/metadata/labels/krateo.io~1composition-version", Value: req.Kind.Version},
					)

					success = true
					return webhook.Patched("mutating webhook called",
						patch...,
					)
				}

				patch = append(patch,
					webhook.JSONPatchOp{Operation: "add", Path: "/metadata/labels/krateo.io~1composition-version", Value: req.Kind.Version},
				)
			}
			success = true
			return webhook.Patched("mutating webhook called",
				patch...,
			)
		}),
	}
}
