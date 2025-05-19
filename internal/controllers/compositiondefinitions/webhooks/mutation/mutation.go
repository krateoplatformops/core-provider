package mutation

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/webhooks/utils/defaults"
	crdtools "github.com/krateoplatformops/core-provider/internal/tools/crd"
	"gomodules.xyz/jsonpatch/v2"
	v1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func NewWebhookHandler(cli client.Client) *webhook.Admission {
	return &webhook.Admission{
		Handler: admission.HandlerFunc(func(ctx context.Context, req webhook.AdmissionRequest) webhook.AdmissionResponse {
			unstructuredObj := &unstructured.Unstructured{}
			if err := json.Unmarshal(req.Object.Raw, unstructuredObj); err != nil {
				return webhook.Errored(http.StatusBadRequest, err)
			}
			// Get CRD from the request
			crd, err := crdtools.Get(ctx, cli, schema.GroupVersionResource{
				Group:    req.Kind.Group,
				Version:  req.Kind.Version,
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
				fmt.Println("Error populating defaults from CRD:", err)
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
				if labels == nil || len(labels) == 0 {
					patch = append(patch,
						webhook.JSONPatchOp{Operation: "add", Path: "/metadata/labels", Value: map[string]string{}},
						webhook.JSONPatchOp{Operation: "add", Path: "/metadata/labels/krateo.io~1composition-version", Value: req.Kind.Version},
					)

					return webhook.Patched("mutating webhook called",
						patch...,
					)
				}

				patch = append(patch,
					webhook.JSONPatchOp{Operation: "add", Path: "/metadata/labels/krateo.io~1composition-version", Value: req.Kind.Version},
				)
			}
			return webhook.Patched("mutating webhook called",
				patch...,
			)
		}),
	}
}
