package main

import (
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/krateoplatformops/core-provider/internal/controllers"
	"github.com/krateoplatformops/core-provider/internal/tools/certs"
	"github.com/krateoplatformops/snowplow/plumbing/env"
	"k8s.io/client-go/kubernetes"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/krateoplatformops/core-provider/apis"
	"github.com/krateoplatformops/provider-runtime/pkg/controller"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"github.com/krateoplatformops/provider-runtime/pkg/ratelimiter"

	"github.com/stoewer/go-strcase"
)

const (
	providerName = "Core"
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
	flag.Parse()

	log.Default().SetOutput(os.Stderr)

	zl := zap.New(zap.UseDevMode(*debug))
	logr := logging.NewLogrLogger(zl.WithName(fmt.Sprintf("%s-provider", strcase.KebabCase(providerName))))
	if *debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		ctrl.SetLogger(zl)
	}

	logr.Debug("Starting", "sync-period", syncPeriod.String(), "poll-interval", pollInterval.String(), "max-reconcile-rate", *maxReconcileRate, "leader-election", *leaderElection)

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		log.Fatalf("Cannot create kubernetes client: %v", err)
		return
	}

	cert, key, err := certs.GenerateClientCertAndKey(client, logr.Debug, certs.GenerateClientCertAndKeyOpts{
		Duration: time.Hour * 1,
		UserID:   "12345", // TODO: use a real user ID - Does core-provider have a user ID?
		Username: "core-provider-webhook-service.demo-system.svc",
		Groups:   []string{"core-provider-webhook-service.demo-system.svc"},
	})
	if err != nil {
		log.Fatalf("Cannot generate client certificate and key: %v", err)
		return
	}

	decCert, err := base64.StdEncoding.DecodeString(cert)
	if err != nil {
		log.Fatalf("Cannot decode certificate: %v", err)
		return
	}
	decKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		log.Fatalf("Cannot decode key: %v", err)
		return
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
			Port: 9443,
			TLSOpts: []func(*tls.Config){
				func(cfg *tls.Config) {
					cfg.GetCertificate = func(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
						parsedKey, err := tls.X509KeyPair(decCert, decKey)
						if err != nil {
							return nil, fmt.Errorf("failed to parse certificate or key: %w", err)
						}
						return &parsedKey, nil
					}
				},
			},
		}),
	})
	if err != nil {
		log.Fatalf("Cannot create controller manager: %v", err)
		return
	}

	err = os.Setenv("TLS_CERT", cert)
	if err != nil {
		log.Fatalf("Cannot set TLS_CERT environment variable: %v", err)
		return
	}

	o := controller.Options{
		Logger:                  logr,
		MaxConcurrentReconciles: *maxReconcileRate,
		PollInterval:            *pollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobalExponential(*minErrorRetryInterval, *maxErrorRetryInterval),
	}

	if err := apis.AddToScheme(mgr.GetScheme()); err != nil {
		log.Fatalf("Cannot add APIs to scheme: %v", err)
		return
	}
	if err := controllers.Setup(mgr, o); err != nil {
		log.Fatalf("Cannot setup controllers: %v", err)
		return
	}
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		log.Fatalf("Cannot start controller manager: %v", err)
		return
	}
}
