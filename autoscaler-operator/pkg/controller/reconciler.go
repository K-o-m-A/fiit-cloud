package controller

import (
	"context"
	"fmt"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	metricsclient "k8s.io/metrics/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/K-o-m-A/fiit-cloud/autoscaler-operator/pkg/metrics"
	"github.com/K-o-m-A/fiit-cloud/autoscaler-operator/pkg/prometheus"
	"github.com/K-o-m-A/fiit-cloud/autoscaler-operator/pkg/scaler"
)

type Options struct {
	SyncPeriod       time.Duration
	MetricsClient    metricsclient.Interface
	PrometheusClient *prometheus.Client
}

type DeploymentReconciler struct {
	client    client.Client
	collector *metrics.Collector

	scaleUpTimes   map[string]time.Time
	scaleDownTimes map[string]time.Time
}

func SetupWithManager(mgr manager.Manager, opts Options) error {
	r := &DeploymentReconciler{
		client:         mgr.GetClient(),
		collector:      metrics.New(mgr.GetClient(), opts.MetricsClient, opts.PrometheusClient),
		scaleUpTimes:   make(map[string]time.Time),
		scaleDownTimes: make(map[string]time.Time),
	}

	labelSelector := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		v, ok := obj.GetLabels()[LabelEnabled]
		return ok && v == "true"
	})

	return ctrlbuilder.ControllerManagedBy(mgr).
		For(&appsv1.Deployment{}, ctrlbuilder.WithPredicates(labelSelector)).
		Complete(r)
}

func (r *DeploymentReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx).WithValues(
		"deployment", req.NamespacedName,
	)

	dep := &appsv1.Deployment{}
	if err := r.client.Get(ctx, req.NamespacedName, dep); err != nil {
		if errors.IsNotFound(err) {
			key := req.NamespacedName.String()
			delete(r.scaleUpTimes, key)
			delete(r.scaleDownTimes, key)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("fetching deployment: %w", err)
	}

	if !dep.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	cfg, err := ParseDeploymentConfig(dep)
	if err != nil {
		logger.Error(err, "invalid autoscaler configuration; skipping")
		r.emitEvent(ctx, dep, corev1.EventTypeWarning, "InvalidConfig", err.Error())
		return reconcile.Result{}, nil
	}

	logger.V(1).Info("resolved config",
		"min", cfg.MinReplicas, "max", cfg.MaxReplicas,
		"cpuEnabled", cfg.CPUEnabled, "memEnabled", cfg.MemEnabled,
	)

	podSelector, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("building pod selector: %w", err)
	}

	snap, metricsErr := r.collector.Collect(
		ctx,
		req.Namespace,
		dep.Name,
		podSelector,
	)
	if metricsErr != nil {
		logger.Error(metricsErr, "partial metrics collection error")
	}

	if snap.PodCount == 0 {
		logger.V(1).Info("no running pods with metrics yet; holding")
		return reconcile.Result{RequeueAfter: 15 * time.Second}, nil
	}

	logger.V(1).Info("metrics snapshot",
		"pods", snap.PodCount,
		"avgCPU", snap.AvgCPUUtilizationPct,
		"avgMem", snap.AvgMemUtilizationPct,
		"avgRPS", snap.AvgRPS,
	)

	key := req.NamespacedName.String()
	currentReplicas := int32(1)
	if dep.Spec.Replicas != nil {
		currentReplicas = *dep.Spec.Replicas
	}

	decision := scaler.Evaluate(scaler.Input{
		CurrentReplicas:      currentReplicas,
		LastScaleUpTime:      r.scaleUpTimes[key],
		LastScaleDownTime:    r.scaleDownTimes[key],
		MinReplicas:          cfg.MinReplicas,
		MaxReplicas:          cfg.MaxReplicas,
		ScaleUpStep:          cfg.ScaleUpStep,
		ScaleDownStep:        cfg.ScaleDownStep,
		ScaleUpCooldownSec:   cfg.ScaleUpCooldownSec,
		ScaleDownCooldownSec: cfg.ScaleDownCooldownSec,
		CPUEnabled:           cfg.CPUEnabled,
		CPUScaleUpPct:        cfg.CPUScaleUpPct,
		CPUScaleDownPct:      cfg.CPUScaleDownPct,
		MemEnabled:            cfg.MemEnabled,
		MemScaleUpPct:         cfg.MemScaleUpPct,
		MemScaleDownPct:       cfg.MemScaleDownPct,
		RPSEnabled:            cfg.RPSEnabled,
		RPSScaleUpThreshold:   cfg.RPSScaleUpThreshold,
		RPSScaleDownThreshold: cfg.RPSScaleDownThreshold,
		Snapshot:              snap,
		Now:                   time.Now(),
	})

	logger.Info("scaling decision", "decision", decision.String())

	if decision.Direction == scaler.Hold {
		return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if err := r.applyScale(ctx, dep, decision.DesiredReplicas); err != nil {
		r.emitEvent(ctx, dep, corev1.EventTypeWarning, "ScaleFailed", err.Error())
		return reconcile.Result{}, fmt.Errorf("applying scale: %w", err)
	}

	now := time.Now()
	switch decision.Direction {
	case scaler.ScaleUp:
		r.scaleUpTimes[key] = now
		r.emitEvent(ctx, dep, corev1.EventTypeNormal, "ScaledUp",
			fmt.Sprintf("Scaled up to %d replicas: %s", decision.DesiredReplicas, decision.String()))
		logger.Info("scaled up", "from", currentReplicas, "to", decision.DesiredReplicas)

	case scaler.ScaleDown:
		r.scaleDownTimes[key] = now
		r.emitEvent(ctx, dep, corev1.EventTypeNormal, "ScaledDown",
			fmt.Sprintf("Scaled down to %d replicas: %s", decision.DesiredReplicas, decision.String()))
		logger.Info("scaled down", "from", currentReplicas, "to", decision.DesiredReplicas)
	}

	return reconcile.Result{RequeueAfter: 30 * time.Second}, nil
}

func (r *DeploymentReconciler) applyScale(ctx context.Context, dep *appsv1.Deployment, desired int32) error {
	patch := client.MergeFrom(dep.DeepCopy())
	dep.Spec.Replicas = &desired
	return r.client.Patch(ctx, dep, patch)
}

func (r *DeploymentReconciler) emitEvent(ctx context.Context, dep *appsv1.Deployment, eventType, reason, msg string) {
	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "autoscaler-",
			Namespace:    dep.Namespace,
		},
		InvolvedObject: corev1.ObjectReference{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
			Name:       dep.Name,
			Namespace:  dep.Namespace,
			UID:        dep.UID,
		},
		Type:    eventType,
		Reason:  reason,
		Message: msg,
		Source: corev1.EventSource{
			Component: "autoscaler-operator",
		},
		FirstTimestamp: metav1.Now(),
		LastTimestamp:  metav1.Now(),
		Count:          1,
	}
	_ = r.client.Create(ctx, event)
}

func labelSelectorToLabelsSelector(s *metav1.LabelSelector) (labels.Selector, error) {
	return metav1.LabelSelectorAsSelector(s)
}
