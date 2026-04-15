package conversion

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/webhooks/utils/convertible"
	webhooktelemetry "github.com/krateoplatformops/core-provider/internal/telemetry/webhooks"
	prettylog "github.com/krateoplatformops/plumbing/slogs/pretty"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"

	apix "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func NewWebhookHandler(scheme *runtime.Scheme, metrics ...*webhooktelemetry.Metrics) http.Handler {
	var recorder *webhooktelemetry.Metrics
	if len(metrics) > 0 {
		recorder = metrics[0]
	}

	lh := prettylog.New(&slog.HandlerOptions{
		Level:     slog.LevelError,
		AddSource: false,
	},
		prettylog.WithDestinationWriter(os.Stderr),
		prettylog.WithColor(),
		prettylog.WithOutputEmptyAttrs(),
	)

	logrlog := logr.FromSlogHandler(slog.New(lh).Handler())
	log := logging.NewLogrLogger(logrlog)

	return &webhook{scheme: scheme, log: log.WithName("core-provider-conversion-webhook"), metrics: recorder}
}

// webhook implements a CRD conversion webhook HTTP handler.
type webhook struct {
	scheme  *runtime.Scheme
	log     logging.Logger
	metrics *webhooktelemetry.Metrics
}

// ensure Webhook implements http.Handler
var _ http.Handler = &webhook{}

func (wh *webhook) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log := wh.log
	started := time.Now()
	success := false
	defer func() {
		if wh.metrics != nil {
			wh.metrics.RecordRequest(r.Context(), "conversion", "convert", time.Since(started), success)
		}
	}()

	convertReview := &apix.ConversionReview{}
	err := json.NewDecoder(r.Body).Decode(convertReview)
	if err != nil {
		log.Error(err, "failed to read conversion request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if convertReview.Request == nil {
		log.Error(nil, "conversion request is nil")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	resp, err := wh.handleConvertRequest(convertReview.Request)
	if err != nil {
		log.Error(err, "failed to convert", "request", convertReview.Request.UID)
		convertReview.Response = errored(err)
	} else {
		convertReview.Response = resp
	}
	convertReview.Response.UID = convertReview.Request.UID
	convertReview.Request = nil

	err = json.NewEncoder(w).Encode(convertReview)
	if err != nil {
		log.Error(err, "failed to write response")
		return
	}

	success = convertReview.Response != nil && convertReview.Response.Result.Status == metav1.StatusSuccess
}

// handles a version conversion request.
func (wh *webhook) handleConvertRequest(req *apix.ConversionRequest) (*apix.ConversionResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("conversion request is nil")
	}
	var objects []runtime.RawExtension

	for _, obj := range req.Objects {
		usrc, gvk, err := unstructured.UnstructuredJSONScheme.Decode(obj.Raw, nil, nil)
		if err != nil {
			return nil, fmt.Errorf("error decoding object: %w", err)
		}
		unssrc, _ := usrc.(*unstructured.Unstructured)
		src := &convertible.Hub{Unstructured: unssrc}
		dst := convertible.CreateEmptyConvertible(req.DesiredAPIVersion, gvk.Kind)
		dst.Object["metadata"] = src.Object["metadata"]
		dst.Object["spec"] = src.Object["spec"]
		dst.Object["status"] = src.Object["status"]
		objects = append(objects, runtime.RawExtension{Object: dst})
	}
	return &apix.ConversionResponse{
		UID:              req.UID,
		ConvertedObjects: objects,
		Result: metav1.Status{
			Status: metav1.StatusSuccess,
		},
	}, nil
}

// helper to construct error response.
func errored(err error) *apix.ConversionResponse {
	return &apix.ConversionResponse{
		Result: metav1.Status{
			Status:  metav1.StatusFailure,
			Message: err.Error(),
		},
	}
}
