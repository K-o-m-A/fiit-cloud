package main

import (
	"flag"
	"os"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"

	"github.com/K-o-m-A/fiit-cloud/autoscaler-operator/pkg/controller"
)

func main() {
	var (
		watchNamespace  string
		syncPeriod      time.Duration
		metricsBindAddr string
		leaderElect     bool
	)

	flag.StringVar(&watchNamespace, "watch-namespace", "",
		"Namespace to watch for labeled Deployments. Empty = all namespaces.")
	flag.DurationVar(&syncPeriod, "sync-period", 30*time.Second,
		"How often the controller re-evaluates scaling decisions.")
	flag.StringVar(&metricsBindAddr, "metrics-bind-address", ":8080",
		"Address for the controller-runtime metrics endpoint.")
	flag.BoolVar(&leaderElect, "leader-elect", false,
		"Enable leader election for high-availability deployments.")

	opts := zap.Options{Development: false}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	log.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
	setupLog := log.Log.WithName("setup")

	cfg, err := config.GetConfig()
	if err != nil {
		setupLog.Error(err, "unable to get kubeconfig")
		os.Exit(1)
	}

	sp := syncPeriod
	mgr, err := manager.New(cfg, manager.Options{
		Namespace:          watchNamespace,
		MetricsBindAddress: metricsBindAddr,
		LeaderElection:     leaderElect,
		LeaderElectionID:   "autoscaler-operator-leader",
		SyncPeriod:         &sp,
	})
	if err != nil {
		setupLog.Error(err, "unable to create manager")
		os.Exit(1)
	}

	// Register types the controller needs to watch/list.
	if err := appsv1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to add apps/v1 to scheme")
		os.Exit(1)
	}
	if err := corev1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to add core/v1 to scheme")
		os.Exit(1)
	}
	if err := metricsv1beta1.AddToScheme(mgr.GetScheme()); err != nil {
		setupLog.Error(err, "unable to add metrics/v1beta1 to scheme")
		os.Exit(1)
	}

	if err := controller.SetupWithManager(mgr, controller.Options{
		SyncPeriod: syncPeriod,
	}); err != nil {
		setupLog.Error(err, "unable to setup autoscaler controller")
		os.Exit(1)
	}

	setupLog.Info("starting autoscaler operator",
		"watchNamespace", watchNamespace,
		"syncPeriod", syncPeriod,
	)

	if err := mgr.Start(signals.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "manager exited with error")
		os.Exit(1)
	}
}
