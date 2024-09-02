package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alecthomas/kingpin/v2"
	"github.com/krateoplatformops/core-provider/internal/controllers"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

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

	var (
		app = kingpin.New(filepath.Base(os.Args[0]), fmt.Sprintf("Krateo %s Provider.", providerName)).
			DefaultEnvars()
		debug = app.Flag("debug", "Run with debug logging.").Short('d').
			OverrideDefaultFromEnvar(fmt.Sprintf("%s_DEBUG", envVarPrefix)).
			Bool()
		syncPeriod = app.Flag("sync", "Controller manager sync period such as 300ms, 1.5h, or 2h45m").Short('s').
				Default("1h").
				Duration()
		pollInterval = app.Flag("poll", "Poll interval controls how often an individual resource should be checked for drift.").
				Default("5m").
				OverrideDefaultFromEnvar(fmt.Sprintf("%s_POLL_INTERVAL", envVarPrefix)).
				Duration()
		maxReconcileRate = app.Flag("max-reconcile-rate", "The global maximum rate per second at which resources may checked for drift from the desired state.").
					Default("3").
					OverrideDefaultFromEnvar(fmt.Sprintf("%s_MAX_RECONCILE_RATE", envVarPrefix)).
					Int()
		leaderElection = app.Flag("leader-election", "Use leader election for the controller manager.").
				Short('l').
				Default("false").
				OverrideDefaultFromEnvar(fmt.Sprintf("%s_LEADER_ELECTION", envVarPrefix)).
				Bool()
	)
	kingpin.MustParse(app.Parse(os.Args[1:]))

	zl := zap.New(zap.UseDevMode(*debug))
	log := logging.NewLogrLogger(zl.WithName(fmt.Sprintf("%s-provider", strcase.KebabCase(providerName))))
	if *debug {
		// The controller-runtime runs with a no-op logger by default. It is
		// *very* verbose even at info level, so we only provide it a real
		// logger when we're running in debug mode.
		ctrl.SetLogger(zl)
	}

	log.Debug("Starting", "sync-period", syncPeriod.String(), "cdc-image-tag", os.Getenv("CDC_IMAGE_TAG"))

	cfg, err := ctrl.GetConfig()
	kingpin.FatalIfError(err, "Cannot get API server rest config")

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		LeaderElection:   *leaderElection,
		LeaderElectionID: fmt.Sprintf("leader-election-%s-provider", strcase.KebabCase(providerName)),
		Cache: cache.Options{
			SyncPeriod: syncPeriod,
		},
		Metrics: metricsserver.Options{
			BindAddress: ":8080",
		},
	})
	kingpin.FatalIfError(err, "Cannot create controller manager")

	o := controller.Options{
		Logger:                  log,
		MaxConcurrentReconciles: *maxReconcileRate,
		PollInterval:            *pollInterval,
		GlobalRateLimiter:       ratelimiter.NewGlobal(*maxReconcileRate),
	}

	kingpin.FatalIfError(apis.AddToScheme(mgr.GetScheme()), "Cannot add APIs to scheme")
	kingpin.FatalIfError(controllers.Setup(mgr, o), "Cannot setup controllers")
	kingpin.FatalIfError(mgr.Start(ctrl.SetupSignalHandler()), "Cannot start controller manager")
}
