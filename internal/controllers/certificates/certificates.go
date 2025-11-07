package certificates

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"

	compositiondefinitionsv1alpha1 "github.com/krateoplatformops/core-provider/apis/compositiondefinitions/v1alpha1"
	crdclient "github.com/krateoplatformops/core-provider/internal/tools/crd"
	crdutils "github.com/krateoplatformops/core-provider/internal/tools/crd/generation"

	"github.com/krateoplatformops/core-provider/internal/tools/pluralizer"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/krateoplatformops/core-provider/internal/tools/certs"
	"github.com/krateoplatformops/core-provider/internal/tools/kube"
	"github.com/krateoplatformops/core-provider/internal/tools/objects"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type CertManager struct {
	kube                        client.Client
	client                      kubernetes.Interface
	certOpts                    certs.GenerateClientCertAndKeyOpts
	log                         func(msg string, keysAndValues ...any)
	pluralizer                  pluralizer.PluralizerInterface
	mutatingWebhookTemplatePath string
	certPath                    string
	caBundle                    []byte
	webhookServiceMeta          types.NamespacedName
}

type Opts struct {
	WebhookServiceName          string
	WebhookServiceNamespace     string
	MutatingWebhookTemplatePath string
	CertOpts                    certs.GenerateClientCertAndKeyOpts
	RestConfig                  *rest.Config
}

func NewCertManager(o Opts, optsFuncs ...FuncOption) (*CertManager, error) {
	if o.RestConfig == nil {
		return nil, fmt.Errorf("rest config cannot be nil")
	}
	if o.MutatingWebhookTemplatePath == "" {
		return nil, fmt.Errorf("mutating webhook template path cannot be empty")
	}
	if o.WebhookServiceName == "" {
		return nil, fmt.Errorf("webhook service name cannot be empty")
	}
	if o.WebhookServiceNamespace == "" {
		return nil, fmt.Errorf("webhook service namespace cannot be empty")
	}
	kube, err := client.New(o.RestConfig, client.Options{})
	if err != nil {
		panic(fmt.Sprintf("error creating kube client: %s", err))
	}
	client, err := kubernetes.NewForConfig(o.RestConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating kubernetes client: %w", err)
	}

	opts := defaultOptions()
	for _, fn := range optsFuncs {
		fn(&opts)
	}

	mgr := &CertManager{
		kube:                        kube,
		client:                      client,
		certOpts:                    o.CertOpts,
		log:                         opts.log,
		pluralizer:                  opts.pluralizer,
		mutatingWebhookTemplatePath: o.MutatingWebhookTemplatePath,
		certPath:                    opts.path,
		webhookServiceMeta: types.NamespacedName{
			Name:      o.WebhookServiceName,
			Namespace: o.WebhookServiceNamespace,
		},
	}
	cabundle, err := os.ReadFile(filepath.Join(opts.path, "tls.crt"))
	if err != nil {
		if os.IsNotExist(err) {
			mgr.RefreshCertificates()
			return mgr, nil
		}
		return nil, fmt.Errorf("error reading CA bundle: %w", err)
	}
	mgr.caBundle = cabundle
	return mgr, nil
}

func (m *CertManager) ManageCertificates(ctx context.Context, gvr schema.GroupVersionResource) error {
	ok, cert, key, err := certs.CheckOrRegenerateClientCertAndKey(m.client, m.log, m.certOpts)
	if err != nil {
		return err
	}

	if !ok {
		m.log("Certificate has been regenerated, updating certificates for webhook server")
		err = certs.UpdateCerts(cert, key, m.certPath)
		if err != nil {
			return err
		}
		m.caBundle, err = getCABundle(m.certPath)
		if err != nil {
			return fmt.Errorf("error getting CA bundle: %w", err)
		}
		m.log("Updating certficates for CRDs and Mutating Webhook Configurations")
	}
	err = m.propagateCABundle(ctx,
		m.caBundle,
		gvr)
	if err != nil {
		return fmt.Errorf("error updating CA bundle: %w", err)
	}
	return nil
}

func (m *CertManager) UpdateExistingResources(ctx context.Context) error {
	var cdList compositiondefinitionsv1alpha1.CompositionDefinitionList
	err := m.kube.List(ctx, &cdList, &client.ListOptions{Namespace: metav1.NamespaceAll})
	if err != nil {
		return fmt.Errorf("error listing CompositionDefinitions: %s", err)
	}
	for i := range cdList.Items {
		cd := &cdList.Items[i]
		if cd.Status.ApiVersion != "" && cd.Status.Kind != "" {
			gvk := schema.FromAPIVersionAndKind(cd.Status.ApiVersion, cd.Status.Kind)
			gvr, err := m.pluralizer.GVKtoGVR(gvk)
			if err != nil {
				return fmt.Errorf("error converting GVK to GVR: %w - GVK: %s", err, gvk.String())
			}

			err = m.propagateCABundle(ctx, m.caBundle, gvr)
			if err != nil {
				return fmt.Errorf("error updating CA bundle: %w", err)
			}
			m.log("Updated CA bundle for CRD and MutatingWebhookConfiguration", "GVR", gvr.String())
		}
	}
	return nil
}

func (m *CertManager) RefreshCertificates() error {
	cert, key, err := certs.GenerateClientCertAndKey(m.client, m.log, m.certOpts)
	if err != nil {
		return fmt.Errorf("error generating client certificate and key: %w", err)
	}
	err = certs.UpdateCerts(cert, key, m.certPath)
	if err != nil {
		return fmt.Errorf("error updating certificates: %w", err)
	}
	m.caBundle, err = getCABundle(m.certPath)
	if err != nil {
		return fmt.Errorf("error getting CA bundle: %w", err)
	}
	return nil
}

func (m *CertManager) GetCertsPath() string {
	return m.certPath
}

func (m *CertManager) GetCABundle() []byte {
	return m.caBundle
}

func (m *CertManager) GetServiceNamespace() string {
	return m.webhookServiceMeta.Namespace
}

func (m *CertManager) GetServiceName() string {
	return m.webhookServiceMeta.Name
}

func getCABundle(certsPath string) ([]byte, error) {
	fb, err := os.ReadFile(filepath.Join(certsPath, "tls.crt"))
	if err != nil {
		return nil, err
	}

	return fb, nil
}

func (m *CertManager) propagateCABundle(ctx context.Context, cabundle []byte, gvr schema.GroupVersionResource) error {
	crd, err := crdclient.Get(ctx, m.kube, gvr.GroupResource())
	if err != nil {
		return fmt.Errorf("error getting CRD: %w", err)
	}

	crd.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apiextensions.k8s.io",
		Kind:    "CustomResourceDefinition",
		Version: "v1",
	})

	if len(crd.Spec.Versions) > 1 {
		m.log("Updating CA bundle for CRD", "Name", crd.Name)
		err = crdutils.UpdateCABundle(crd, cabundle)
		if err != nil {
			return fmt.Errorf("error updating CA bundle: %w", err)
		}
		// Update the CRD with the new CA bundle
		err = kube.Apply(ctx, m.kube, crd, kube.ApplyOptions{})
		if err != nil {
			return fmt.Errorf("error applying CRD: %w", err)
		}
	}

	// Update the mutating webhook config with the new CA bundle
	mutatingWebhookConfig := admissionregistrationv1.MutatingWebhookConfiguration{}
	err = objects.CreateK8sObject(&mutatingWebhookConfig,
		schema.GroupVersionResource{},
		types.NamespacedName{},
		m.mutatingWebhookTemplatePath,
		"caBundle", base64.StdEncoding.EncodeToString(cabundle))
	if err != nil {
		return fmt.Errorf("error creating mutating webhook config: %w", err)
	}
	m.log("Updating CA bundle for MutatingWebhookConfiguration", "Name", mutatingWebhookConfig.Name)
	err = kube.Apply(ctx, m.kube, &mutatingWebhookConfig, kube.ApplyOptions{})
	if err != nil {
		return fmt.Errorf("error applying mutating webhook config: %w", err)
	}

	return nil
}
