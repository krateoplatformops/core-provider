package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/go-logr/logr"

	"github.com/krateoplatformops/core-provider/internal/controllers"
	"github.com/krateoplatformops/core-provider/internal/controllers/compositiondefinitions"
	"github.com/krateoplatformops/core-provider/internal/tools/certs"
	"github.com/krateoplatformops/core-provider/internal/tools/chart/chartfs"
	"github.com/krateoplatformops/plumbing/env"
	prettylog "github.com/krateoplatformops/plumbing/slogs/pretty"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/krateoplatformops/core-provider/apis"
	"github.com/krateoplatformops/provider-runtime/pkg/controller"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"github.com/krateoplatformops/provider-runtime/pkg/ratelimiter"

	"github.com/stoewer/go-strcase"
)

const (
	providerName                 = "Core"
	helmRegistryConfigPathEnvVar = "HELM_REGISTRY_CONFIG_PATH"
)

func main() {
	envVarPrefix := fmt.Sprintf("%s_PROVIDER", strcase.UpperSnakeCase(providerName))

	debug := flag.Bool("debug", env.Bool(fmt.Sprintf("%s_DEBUG", envVarPrefix), false), "Run with debug logging.")
	syncPeriod := flag.Duration("sync", env.Duration(fmt.Sprintf("%s_SYNC", envVarPrefix), time.Hour*1), "Controller manager sync period such as 300ms, 1.5h, or 2h45m")
	pollInterval := flag.Duration("poll", env.Duration(fmt.Sprintf("%s_POLL_INTERVAL", envVarPrefix), time.Minute*3), "Poll interval controls how often an individual resource should be checked for drift.")
	maxReconcileRate := flag.Int("max-reconcile-rate", env.Int(fmt.Sprintf("%s_MAX_RECONCILE_RATE", envVarPrefix), 3), "The global maximum rate per second at which resources may checked for drift from the desired state.")
	leaderElection := flag.Bool("leader-election", env.Bool(fmt.Sprintf("%s_LEADER_ELECTION", envVarPrefix), false), "Use leader election for the controller manager.")
	maxErrorRetryInterval := flag.Duration("max-error-retry-interval", env.Duration(fmt.Sprintf("%s_MAX_ERROR_RETRY_INTERVAL", envVarPrefix), 1*time.Minute), "The maximum interval between retries when an error occurs. This should be less than the half of the poll interval.")
	minErrorRetryInterval := flag.Duration("min-error-retry-interval", env.Duration(fmt.Sprintf("%s_MIN_ERROR_RETRY_INTERVAL", envVarPrefix), 1*time.Second), "The minimum interval between retries when an error occurs. This should be less than max-error-retry-interval.")
	webhookServiceName := flag.String("webhook-service-name", env.String(fmt.Sprintf("%s_WEBHOOK_SERVICE_NAME", envVarPrefix), "core-provider-webhook-service"), "The name of the webhook service.")
	webhookServiceNamespace := flag.String("webhook-service-namespace", env.String(fmt.Sprintf("%s_WEBHOOK_SERVICE_NAMESPACE", envVarPrefix), "demo-system"), "The namespace of the webhook service.")
	helmRegistryConfigPath := flag.String("helm-registry-config-path", env.String(helmRegistryConfigPathEnvVar, chartfs.HelmRegistryConfigPathDefault), "The path to the helm registry config file.")
	tlsCertificateDuration := flag.Duration("tls-certificate-duration", env.Duration(fmt.Sprintf("%s_TLS_CERTIFICATE_DURATION", envVarPrefix), 24*time.Hour), "The duration of the TLS certificate. It should be at least 10 minutes and a minimum of 3 times the poll interval.")
	tlsCertificateLeaseExpirationMargin := flag.Duration(
		"tls-certificate-lease-expiration-margin",
		env.Duration(fmt.Sprintf("%s_TLS_CERTIFICATE_LEASE_EXPIRATION_MARGIN", envVarPrefix),
			16*time.Hour),
		"The duration of the TLS certificate lease expiration margin. It represents the time before the certificate expires when the lease should be renewed. It must be less than the TLS certificate duration. Consider values of 2/3 or less of the TLS certificate duration.")
	flag.Parse()

	log.Default().SetOutput(os.Stderr)

	if *tlsCertificateDuration < time.Minute*10 {
		log.Fatalf("The TLS certificate duration must be at least 10 minutes.")
		return
	}
	if *tlsCertificateDuration < time.Duration(3)*(*pollInterval) {
		log.Fatalf("The TLS certificate duration must be at least 3 times the poll interval.")
		return
	}
	if *tlsCertificateLeaseExpirationMargin > *tlsCertificateDuration {
		log.Fatalf("The TLS certificate lease expiration margin must be less than the TLS certificate duration.")
		return
	}

	logLevel := slog.LevelInfo
	if *debug {
		logLevel = slog.LevelDebug
	}

	lh := prettylog.New(&slog.HandlerOptions{
		Level:     logLevel,
		AddSource: false,
	},
		prettylog.WithDestinationWriter(os.Stderr),
		prettylog.WithColor(),
		prettylog.WithOutputEmptyAttrs(),
	)

	logrlog := logr.FromSlogHandler(slog.New(lh).Handler())
	log := logging.NewLogrLogger(logrlog)

	ctrl.SetLogger(logrlog)

	log.Debug("Starting",
		"sync-period", syncPeriod.String(),
		"poll-interval", pollInterval.String(),
		"max-reconcile-rate", *maxReconcileRate,
		"leader-election", *leaderElection,
		"max-error-retry-interval", maxErrorRetryInterval.String(),
		"min-error-retry-interval", minErrorRetryInterval.String(),
		"webhook-service-name", *webhookServiceName,
		"webhook-service-namespace", *webhookServiceNamespace,
		"tls-certificate-duration", tlsCertificateDuration.String(),
		"tls-certificate-lease-expiration-margin", tlsCertificateLeaseExpirationMargin.String(),
		"helm-registry-config-path", *helmRegistryConfigPath)

	cfg, err := ctrl.GetConfig()
	if err != nil {
		log.Info("Cannot get API server rest config", "error", err)
		os.Exit(1)
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Info("Cannot create kubernetes client", "error", err)
		os.Exit(1)
	}

	compositiondefinitions.WebhookServiceName = *webhookServiceName
	compositiondefinitions.WebhookServiceNamespace = *webhookServiceNamespace
	chartfs.HelmRegistryConfigPath = *helmRegistryConfigPath
	compositiondefinitions.CertOpts = certs.GenerateClientCertAndKeyOpts{
		Duration:              *tlsCertificateDuration,
		Username:              fmt.Sprintf("%s.%s.svc", compositiondefinitions.WebhookServiceName, compositiondefinitions.WebhookServiceNamespace),
		Approver:              strcase.KebabCase(envVarPrefix),
		LeaseExpirationMargin: *tlsCertificateLeaseExpirationMargin,
	}

	cert, key, err := certs.GenerateClientCertAndKey(client, log.Debug, compositiondefinitions.CertOpts)
	if err != nil {
		log.Info("Cannot generate client certificate and key", "error", err)
		os.Exit(1)
	}
	err = certs.UpdateCerts(cert, key, compositiondefinitions.CertsPath)
	if err != nil {
		log.Info("Cannot update certificates", "error", err)
		os.Exit(1)
	}

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:   *leaderElection,
		LeaderElectionID: fmt.Sprintf("leader-election-%s-provider", strcase.KebabCase(providerName)),
		Cache: cache.Options{
			SyncPeriod: syncPeriod,
		},
		Metrics: metricsserver.Options{
			BindAddress: ":8080",
		},
		WebhookServer: webhook.NewServer(webhook.Options{
			Port:     9443,
			CertDir:  compositiondefinitions.CertsPath,
			CertName: "tls.crt",
			KeyName:  "tls.key",
		}),
	})
	if err != nil {
		log.Info("Cannot create controller manager", "error", err)
		os.Exit(1)
	}

	o := controller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: *maxReconcileRate,
		PollInterval:            *pollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobalExponential(*minErrorRetryInterval, *maxErrorRetryInterval),
	}

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Info("Cannot add APIs to scheme", "error", err)
		os.Exit(1)
	}
	if err := controllers.Setup(mgr, o); err != nil {
		log.Info("Cannot setup controllers", "error", err)
		os.Exit(1)
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Info("Cannot start controller manager", "error", err)
		os.Exit(1)
	}
}
