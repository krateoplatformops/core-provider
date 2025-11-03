//go:build integration
// +build integration

package compositiondefinitions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/krateoplatformops/plumbing/cache"

	"github.com/go-logr/logr"
	prettylog "github.com/krateoplatformops/plumbing/slogs/pretty"
	"github.com/krateoplatformops/provider-runtime/pkg/controller"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"github.com/krateoplatformops/provider-runtime/pkg/ratelimiter"
	"github.com/stoewer/go-strcase"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/krateoplatformops/core-provider/apis"
	"github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions/certificates"
	"github.com/krateoplatformops/core-provider/internal/tools/certs"
	"github.com/krateoplatformops/plumbing/e2e"
	xenv "github.com/krateoplatformops/plumbing/env"
	rtv1 "github.com/krateoplatformops/provider-runtime/apis/common/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	"sigs.k8s.io/e2e-framework/klient/decoder"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/k8s/resources"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/support/kind"
)

var (
	testenv     env.Environment
	clusterName string
)

const (
	crdPath       = "../../../crds"
	testdataPath  = "../../../testdata/test"
	manifestsPath = "./testdata/manifests"

	namespace = "test-system"
)

func TestMain(m *testing.M) {
	xenv.SetTestMode(true)

	clusterName = "krateo-core-provider-controller"
	testenv = env.New()
	kindCluster := kind.NewCluster(clusterName)

	cleanAssetFolder := func(ctx context.Context, c *envconf.Config) (context.Context, error) {
		err := os.RemoveAll(filepath.Join(os.TempDir(), "assets"))
		return ctx, err
	}

	testenv.Setup(
		cleanAssetFolder,
		envfuncs.CreateCluster(kindCluster, clusterName),
		e2e.CreateNamespace(namespace),

		func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				return ctx, err
			}
			r.WithNamespace(namespace)

			err = os.MkdirAll(filepath.Join(os.TempDir(), "assets", "mutating-webhook-configuration"), os.ModePerm)
			if err != nil {
				return ctx, err
			}
			err = os.Link(filepath.Join(manifestsPath, "mutating-webhook.yaml"), filepath.Join(os.TempDir(), "assets", "mutating-webhook-configuration", "mutating-webhook.yaml"))
			if err != nil {
				return ctx, err
			}

			err = os.MkdirAll(filepath.Join(os.TempDir(), "assets", "cdc-deployment"), os.ModePerm)
			if err != nil {
				return ctx, err
			}
			err = os.Link(filepath.Join(manifestsPath, "deployment.yaml"), filepath.Join(os.TempDir(), "assets", "cdc-deployment", "deployment.yaml"))
			if err != nil {
				return ctx, err
			}

			err = os.MkdirAll(filepath.Join(os.TempDir(), "assets", "cdc-configmap"), os.ModePerm)
			if err != nil {
				return ctx, err
			}
			err = os.Link(filepath.Join(manifestsPath, "configmap.yaml"), filepath.Join(os.TempDir(), "assets", "cdc-configmap", "configmap.yaml"))
			if err != nil {
				return ctx, err
			}

			err = os.MkdirAll(filepath.Join(os.TempDir(), "assets", "json-schema-configmap"), os.ModePerm)
			if err != nil {
				return ctx, err
			}
			err = os.Link(filepath.Join(manifestsPath, "json-schema-configmap.yaml"), filepath.Join(os.TempDir(), "assets", "json-schema-configmap", "configmap.yaml"))
			if err != nil {
				return ctx, err
			}

			err = os.MkdirAll(filepath.Join(os.TempDir(), "assets", "cdc-service"), os.ModePerm)
			if err != nil {
				return ctx, err
			}
			err = os.Link(filepath.Join(manifestsPath, "service.yaml"), filepath.Join(os.TempDir(), "assets", "cdc-service", "service.yaml"))
			if err != nil {
				return ctx, err
			}

			err = os.MkdirAll(filepath.Join(os.TempDir(), "assets", "cdc-rbac"), os.ModePerm)
			if err != nil {
				return ctx, err
			}

			err = os.Link(filepath.Join(manifestsPath, "rbac", "clusterrole.yaml"), filepath.Join(os.TempDir(), "assets", "cdc-rbac", "clusterrole.yaml"))
			if err != nil {
				return ctx, err
			}
			err = os.Link(filepath.Join(manifestsPath, "rbac", "clusterrolebinding.yaml"), filepath.Join(os.TempDir(), "assets", "cdc-rbac", "clusterrolebinding.yaml"))
			if err != nil {
				return ctx, err
			}
			err = os.Link(filepath.Join(manifestsPath, "rbac", "serviceaccount.yaml"), filepath.Join(os.TempDir(), "assets", "cdc-rbac", "serviceaccount.yaml"))
			if err != nil {
				return ctx, err
			}
			err = os.Link(filepath.Join(manifestsPath, "rbac", "secret-role.yaml"), filepath.Join(os.TempDir(), "assets", "cdc-rbac", "secret-role.yaml"))
			if err != nil {
				return ctx, err
			}
			err = os.Link(filepath.Join(manifestsPath, "rbac", "secret-rolebinding.yaml"), filepath.Join(os.TempDir(), "assets", "cdc-rbac", "secret-rolebinding.yaml"))
			if err != nil {
				return ctx, err
			}
			err = os.Link(filepath.Join(manifestsPath, "rbac", "compositiondefinition-role.yaml"), filepath.Join(os.TempDir(), "assets", "cdc-rbac", "compositiondefinition-role.yaml"))
			if err != nil {
				return ctx, err
			}
			err = os.Link(filepath.Join(manifestsPath, "rbac", "compositiondefinition-rolebinding.yaml"), filepath.Join(os.TempDir(), "assets", "cdc-rbac", "compositiondefinition-rolebinding.yaml"))
			if err != nil {
				return ctx, err
			}

			// Install CRDs
			err = decoder.DecodeEachFile(
				ctx, os.DirFS(filepath.Join(crdPath)), "*.yaml",
				decoder.CreateIgnoreAlreadyExists(r),
			)

			time.Sleep(2 * time.Second) // wait for the compositiondefinition CRD to be registered

			return ctx, nil
		},
	).Finish(
		envfuncs.DeleteNamespace(namespace),
		envfuncs.TeardownCRDs(crdPath, "core.krateo.io_compositiondefinitions.yaml"),
		envfuncs.DestroyCluster(clusterName),
		cleanAssetFolder,
		func(ctx context.Context, c *envconf.Config) (context.Context, error) {
			if v := ctx.Value(stopKey{}); v != nil {
				if stop, ok := v.(context.CancelFunc); ok {
					fmt.Println("Stopping controller manager at ", time.Now().String())
					stop() // stops mgr.Start and the background goroutine
				}
			}
			return ctx, nil
		},
	)

	os.Exit(testenv.Run(m))
}

type stopKey struct{} // key for storing stop func in ctx

func TestController(t *testing.T) {
	os.Setenv("DEBUG", "1")

	setupController := func(ctx context.Context, cfg *envconf.Config) (context.Context, error) {
		lh := prettylog.New(&slog.HandlerOptions{
			Level:     slog.LevelDebug,
			AddSource: false,
		},
			prettylog.WithDestinationWriter(os.Stderr),
			prettylog.WithColor(),
			prettylog.WithOutputEmptyAttrs(),
		)

		logrlog := logr.FromSlogHandler(slog.New(lh).Handler())
		log := logging.NewLogrLogger(logrlog)

		pluralizer := NewTestPluralizer(false)
		webhookServiceName := "core-provider-webhook-service"
		webhookServiceNamespace := "krateo-system"
		ctrl.SetLogger(logrlog)

		certOpts := certs.GenerateClientCertAndKeyOpts{
			Duration:              24 * time.Hour,
			Username:              fmt.Sprintf("%s.%s.svc", webhookServiceName, webhookServiceNamespace),
			Approver:              strcase.KebabCase("core-provider"),
			LeaseExpirationMargin: 16 * time.Hour,
		}

		certMgr, err := certificates.NewCertManager(certificates.Opts{
			WebhookServiceName:          webhookServiceName,
			WebhookServiceNamespace:     webhookServiceNamespace,
			MutatingWebhookTemplatePath: MutatingWebhookPath,
			CertOpts:                    certOpts,
			RestConfig:                  cfg.Client().RESTConfig(),
		}, certificates.WithPluralizer(pluralizer))
		if err != nil {
			log.Info("Cannot create certificate manager", "error", err)
			os.Exit(1)
		}
		err = certMgr.RefreshCertificates()
		if err != nil {
			log.Info("Cannot refresh certificates", "error", err)
			os.Exit(1)
		}
		mgr, err := ctrl.NewManager(cfg.Client().RESTConfig(), ctrl.Options{
			Metrics: server.Options{
				BindAddress: "0", // disable metrics for tests
			},
			WebhookServer: webhook.NewServer(webhook.Options{
				Port:     9443,
				CertDir:  CertsPath,
				CertName: "tls.crt",
				KeyName:  "tls.key",
			}),
		})
		if err != nil {
			return ctx, err
		}

		o := controller.Options{
			Logger:                  log,
			MaxConcurrentReconciles: 1,
			PollInterval:            20 * time.Second,
			GlobalRateLimiter:       ratelimiter.NewGlobalExponential(1*time.Second, 1*time.Minute),
		}

		if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
			log.Info("Cannot add APIs to scheme", "error", err)
			os.Exit(1)
		}

		if err := Setup(mgr, Options{
			ControllerOptions: o,
			CertManager:       certMgr,
			Pluralizer:        pluralizer,
		}); err != nil {
			log.Info("Cannot setup controllers", "error", err)
			os.Exit(1)
		}
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			log.Info("Cannot start controller manager", "error", err)
			os.Exit(1)
		}
		return ctx, nil
	}
	getCDLI := func(ctx context.Context) []v1alpha1.CompositionDefinition {
		fli, err := decoder.DecodeAllFiles(ctx, os.DirFS(testdataPath), "*.yaml", decoder.MutateNamespace(namespace))
		if err != nil {
			t.Fatal(err)
		}
		var cdli []v1alpha1.CompositionDefinition
		for _, fi := range fli {
			cd, ok := fi.(*v1alpha1.CompositionDefinition)
			if !ok {
				t.Fatalf("expected CompositionDefinition, got %T", fi)
			}
			cdli = append(cdli, *cd)
		}
		return cdli
	}

	f := features.New("Setup").
		Setup(func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			stopCtx, stop := context.WithCancel(context.Background())

			go func() {
				_, err := setupController(stopCtx, cfg)
				if err != nil {
					fmt.Printf("Error starting controller manager: %v\n", err)
				}
			}()

			// store cancel func in returned context so callers can stop the manager
			ctx = context.WithValue(ctx, stopKey{}, stop)
			return ctx
		}).
		Assess("Test Create", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			r, err := resources.New(cfg.Client().RESTConfig())
			if err != nil {
				t.Fail()
			}

			apis.AddToScheme(r.GetScheme())
			r.WithNamespace(namespace)

			cdli := getCDLI(ctx)
			for _, res := range cdli {
				// Create CompositionDefinition
				err = r.Create(ctx, &res)
				if err != nil {
					t.Fatal(err)
				}
			}

			for _, res := range cdli {
				//wait for resource to be created
				if err := wait.For(
					conditions.New(r).ResourceMatch(&res, func(object k8s.Object) bool {
						mg := object.(*v1alpha1.CompositionDefinition)
						return mg.GetCondition(rtv1.TypeReady).Reason == rtv1.ReasonAvailable && mg.GetCondition(rtv1.TypeReady).Status == metav1.ConditionTrue
					}),
					wait.WithTimeout(5*time.Minute),
					wait.WithInterval(5*time.Second),
				); err != nil {
					obj := v1alpha1.CompositionDefinition{}
					r.Get(ctx, res.Name, namespace, &obj)
					b, _ := json.MarshalIndent(obj.Status, "", "  ")
					t.Logf("CompositionDefinition Status: %s", string(b))
					t.Fatal(err)
				}
			}
			return ctx
		}).Assess("Test Change Version", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		toUpgrade := []struct {
			filename       string
			currentVersion string
			newVersion     string
		}{
			{
				filename:       "fireworksapp.yaml",
				currentVersion: "2.0.3",
				newVersion:     "2.0.4",
			},
		}

		r, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			t.Fail()
		}
		apis.AddToScheme(r.GetScheme())
		r.WithNamespace(namespace)

		for _, test := range toUpgrade {
			var res v1alpha1.CompositionDefinition
			err = decoder.DecodeFile(
				os.DirFS(filepath.Join(testdataPath)), test.filename,
				&res,
				decoder.MutateNamespace(namespace),
			)
			if err != nil {
				t.Fatal(err)
			}

			err = r.Get(ctx, res.Name, namespace, &res)
			if err != nil {
				t.Fatal(err)
			}

			res.Spec.Chart.Version = test.newVersion
			// Update CompositionDefinition
			err = r.Update(ctx, &res)
			if err != nil {
				t.Fatal(err)
			}
		}

		for _, test := range toUpgrade {
			oldVersionNormalized := normalizeVersion(test.currentVersion, '-')
			newVersionNormalized := normalizeVersion(test.newVersion, '-')
			var res v1alpha1.CompositionDefinition
			err = decoder.DecodeFile(
				os.DirFS(filepath.Join(testdataPath)), test.filename,
				&res,
				decoder.MutateNamespace(namespace),
			)
			if err != nil {
				t.Fatal(err)
			}

			t.Logf("Checking CompositionDefinition %s for upgrade to version %s", test.filename, newVersionNormalized)

			//wait for resource to be created
			if err := wait.For(
				conditions.New(r).ResourceMatch(&res, func(object k8s.Object) bool {
					t.Logf("Checking CompositionDefinition %s for new version %s", test.filename, newVersionNormalized)
					b, _ := json.MarshalIndent(object, "", "  ")
					t.Logf("CompositionDefinition Object: %s", string(b))
					mg := object.(*v1alpha1.CompositionDefinition)
					return mg.GetCondition(rtv1.TypeReady).Reason == rtv1.ReasonAvailable &&
						len(mg.Status.Managed.VersionInfo) == 3 &&
						slices.ContainsFunc(mg.Status.Managed.VersionInfo, func(v v1alpha1.VersionDetail) bool {
							t.Logf("Checking version %s against new version %s", v.Version, newVersionNormalized)
							return v.Version == newVersionNormalized
						})
				}),
				wait.WithTimeout(15*time.Minute),
				wait.WithInterval(15*time.Second),
			); err != nil {
				obj := v1alpha1.CompositionDefinition{}
				r.Get(ctx, res.Name, namespace, &obj)
				b, _ := json.MarshalIndent(obj.Status, "", "  ")
				t.Logf("CompositionDefinition Status: %s", string(b))
				t.Fatal(err)
			}

			var crd apiextensionsv1.CustomResourceDefinition

			apiextensionsv1.AddToScheme(r.GetScheme())

			gv, err := schema.ParseGroupVersion(res.Status.ApiVersion)
			if err != nil {
				t.Fatal(err)
			}
			crdName := fmt.Sprintf("%s.%s", res.Status.Resource, gv.Group)
			err = r.Get(ctx, crdName, "", &crd)
			if err != nil {
				t.Fatal(err)
			}

			// Check CRD version
			if len(crd.Spec.Versions) != 3 {
				t.Fatalf("Expected 3 versions, got %d", len(crd.Spec.Versions))
			}
			if !slices.ContainsFunc(crd.Spec.Versions, func(v apiextensionsv1.CustomResourceDefinitionVersion) bool {
				return v.Name == newVersionNormalized
			}) {
				t.Fatalf("Expected version %s, got %v", newVersionNormalized, crd.Spec.Versions)
			}
			if !slices.ContainsFunc(crd.Spec.Versions, func(v apiextensionsv1.CustomResourceDefinitionVersion) bool {
				return v.Name == oldVersionNormalized
			}) {
				t.Fatalf("Expected version %s, got %v", oldVersionNormalized, crd.Spec.Versions)
			}
			if !slices.ContainsFunc(crd.Spec.Versions, func(v apiextensionsv1.CustomResourceDefinitionVersion) bool {
				return v.Name == "vacuum"
			}) {
				t.Fatalf("Expected version vacuum, got %v", crd.Spec.Versions)
			}
		}

		return ctx
	}).Assess("Test Delete", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
		r, err := resources.New(cfg.Client().RESTConfig())
		if err != nil {
			t.Fail()
		}
		apis.AddToScheme(r.GetScheme())
		r.WithNamespace(namespace)

		cdli := getCDLI(ctx)

		for _, res := range cdli {
			// Delete CompositionDefinition
			err = r.Delete(ctx, &res)
			if err != nil {
				t.Fatal(err)
			}
		}

		for _, res := range cdli {
			//wait for resource to be deleted
			if err := wait.For(
				conditions.New(r).ResourceDeleted(&res),
				wait.WithTimeout(5*time.Minute),
				wait.WithInterval(5*time.Second),
			); err != nil {
				t.Fatal(err)
			}
		}

		return ctx
	}).Feature()

	testenv.Test(t, f)
}

// This is needed to avoid avoid running the controller in a pod as required by the original plumbings/plurals package
type TestPluralizer struct {
	cache *cache.TTLCache[string, Info]
}

func NewTestPluralizer(cached bool) *TestPluralizer {
	if cached {
		return &TestPluralizer{
			cache: cache.NewTTL[string, Info](),
		}
	}

	return &TestPluralizer{}
}

func (p TestPluralizer) GVKtoGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	info, err := Get(gvk, GetOptions{
		Cache: p.cache,
	})
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: info.Plural,
	}, nil
}

type GetOptions struct {
	Logger       *slog.Logger
	Cache        *cache.TTLCache[string, Info]
	ResolverFunc func(schema.GroupVersionKind) (Info, error)
}

func Get(gvk schema.GroupVersionKind, opts GetOptions) (Info, error) {
	if opts.ResolverFunc == nil {
		opts.ResolverFunc = ResolveAPINames
	}

	var (
		tmp Info
		ok  bool
	)

	if opts.Cache != nil {
		tmp, ok = opts.Cache.Get(gvk.String())
		if ok && opts.Logger != nil {
			opts.Logger.Debug("cache hit", slog.String("gvk", gvk.String()))
		}
	}

	if !ok {
		if opts.Logger != nil {
			opts.Logger.Debug("cache miss", slog.String("gvk", gvk.String()))
		}

		var err error
		tmp, err = opts.ResolverFunc(gvk)
		if err != nil {
			if opts.Logger != nil {
				opts.Logger.Error("unable to discover API names",
					slog.String("gvk", gvk.String()), slog.Any("err", err))
			}
			return Info{}, err
		}

		if opts.Cache != nil {
			opts.Cache.Set(gvk.String(), tmp, time.Hour*48)
		}
	}

	if len(tmp.Plural) == 0 {
		err := &apierrors.StatusError{
			ErrStatus: metav1.Status{
				Status: metav1.StatusFailure,
				Code:   http.StatusNotFound,
				Reason: metav1.StatusReasonNotFound,
				Details: &metav1.StatusDetails{
					Group: gvk.Group,
					Kind:  gvk.Kind,
				},
				Message: fmt.Sprintf("no names found for %q", gvk.GroupVersion().String()),
			}}

		if opts.Logger != nil {
			opts.Logger.Warn(err.ErrStatus.Message)
		}
		return tmp, err
	}

	return tmp, nil
}

func ResolveAPINames(gvk schema.GroupVersionKind) (Info, error) {
	rc, err := rest.InClusterConfig()
	if err != nil {
		if errors.Is(err, rest.ErrNotInCluster) {
			// For testing purposes, use default kubeconfig
			rc, err = clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
			if err != nil {
				return Info{}, err
			}
		} else {
			return Info{}, err
		}
	}

	dc, err := discovery.NewDiscoveryClientForConfig(rc)
	if err != nil {
		return Info{}, err
	}

	list, err := dc.ServerResourcesForGroupVersion(gvk.GroupVersion().String())
	if err != nil {
		return Info{}, err
	}

	if list == nil || len(list.APIResources) == 0 {
		return Info{}, nil
	}

	var tmp Info
	for _, el := range list.APIResources {
		if el.Kind != gvk.Kind {
			continue
		}

		tmp = Info{
			Plural:   el.Name,
			Singular: el.SingularName,
			Shorts:   el.ShortNames,
		}
		break
	}

	return tmp, nil
}

type Info struct {
	Plural   string   `json:"plural"`
	Singular string   `json:"singular"`
	Shorts   []string `json:"shorts"`
}

// For go package and k8s version must complain to this pattern:
//
// [a-z]([-a-z0-9]*[a-z0-9])?
//
// Go package folders allow only underscore char ('_')
// K8s CRD version allow only dash char ('-')
// This version is taken from https://github.com/krateoplatformops/crdgen/blob/bf775894a752cc14d45d0b1f2a9dc080e0277517/internal/coders/support.go
func normalizeVersion(ver string, replaceChar rune) string {
	ver = strings.ToLower(ver)

	// Sostituisce tutti i caratteri non alfanumerici con replaceChar
	re := regexp.MustCompile(`[^a-z0-9]+`)
	ver = re.ReplaceAllString(ver, string(replaceChar))

	// Rimuove caratteri speciali all'inizio e alla fine
	ver = strings.Trim(ver, string(replaceChar))

	// Assicura che inizi con una lettera
	if len(ver) > 0 && ver[0] >= '0' && ver[0] <= '9' {
		ver = "v" + ver
	}

	return ver
}
