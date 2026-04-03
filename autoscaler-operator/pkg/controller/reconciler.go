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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	ctrlbuilder "sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"github.com/K-o-m-A/fiit-cloud/autoscaler-operator/pkg/metrics"
	"github.com/K-o-m-A/fiit-cloud/autoscaler-operator/pkg/scaler"
)

// Options are passed from main to SetupWithManager.
type Options struct {
	SyncPeriod time.Duration
}

// DeploymentReconciler reconciles Deployments that carry the autoscaler opt-in label.
type DeploymentReconciler struct {
	client    client.Client
	collector *metrics.Collector

	// scaleUpTimes and scaleDownTimes track the last scaling event per Deployment
	// (namespace/name key) for cooldown enforcement.
	scaleUpTimes   map[string]time.Time
	scaleDownTimes map[string]time.Time
}

// SetupWithManager registers the reconciler and a Watch on labeled Deployments.
func SetupWithManager(mgr manager.Manager, opts Options) error {
	r := &DeploymentReconciler{
		client:         mgr.GetClient(),
		collector:      metrics.New(mgr.GetClient()),
		scaleUpTimes:   make(map[string]time.Time),
		scaleDownTimes: make(map[string]time.Time),
	}

	// Only reconcile Deployments that carry our opt-in label.
	labelSelector := predicate.NewPredicateFuncs(func(obj client.Object) bool {
		v, ok := obj.GetLabels()[LabelEnabled]
		return ok && v == "true"
	})

	return ctrlbuilder.ControllerManagedBy(mgr).
		For(&appsv1.Deployment{}, ctrlbuilder.WithPredicates(labelSelector)).
		Complete(r)
}

// Reconcile is called by controller-runtime whenever a labeled Deployment changes
// or the sync period elapses.
func (r *DeploymentReconciler) Reconcile(ctx context.Context, req reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx).WithValues(
		"deployment", req.NamespacedName,
	)

	// --- 1. Fetch the Deployment ---
	dep := &appsv1.Deployment{}
	if err := r.client.Get(ctx, req.NamespacedName, dep); err != nil {
		if errors.IsNotFound(err) {
			// Deployment deleted; clean up our in-memory cooldown state.
			key := req.NamespacedName.String()
			delete(r.scaleUpTimes, key)
			delete(r.scaleDownTimes, key)
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, fmt.Errorf("fetching deployment: %w", err)
	}

	// Honour DeletionTimestamp – skip reconciliation for terminating objects.
	if !dep.DeletionTimestamp.IsZero() {
		return reconcile.Result{}, nil
	}

	// --- 2. Parse annotation-driven configuration ---
	cfg, err := ParseDeploymentConfig(dep)
	if err != nil {
		logger.Error(err, "invalid autoscaler configuration; skipping")
		r.emitEvent(ctx, dep, corev1.EventTypeWarning, "InvalidConfig", err.Error())
		// Don't requeue automatically – user must fix the annotation.
		return reconcile.Result{}, nil
	}

	logger.V(1).Info("resolved config",
		"min", cfg.MinReplicas, "max", cfg.MaxReplicas,
		"cpuEnabled", cfg.CPUEnabled, "memEnabled", cfg.MemEnabled,
	)

	// --- 3. Collect metrics ---
	podSelector, err := metav1.LabelSelectorAsSelector(dep.Spec.Selector)
	if err != nil {
		return reconcile.Result{}, fmt.Errorf("building pod selector: %w", err)
	}

	snap, metricsErr := r.collector.Collect(
		ctx,
		req.Namespace,
		podSelector,
	)
	if metricsErr != nil {
		// Log but continue; we may still be able to make a decision with partial data.
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
	)

	// --- 4. Make scaling decision ---
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
		MemEnabled:           cfg.MemEnabled,
		MemScaleUpPct:        cfg.MemScaleUpPct,
		MemScaleDownPct:      cfg.MemScaleDownPct,
		Snapshot:             snap,
		Now:                  time.Now(),
	})

	logger.Info("scaling decision", "decision", decision.String())

	// --- 5. Apply the decision ---
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

// applyScale patches the Deployment's replica count using a strategic merge patch.
func (r *DeploymentReconciler) applyScale(ctx context.Context, dep *appsv1.Deployment, desired int32) error {
	patch := client.MergeFrom(dep.DeepCopy())
	dep.Spec.Replicas = &desired
	return r.client.Patch(ctx, dep, patch)
}

// emitEvent records a Kubernetes Event on the Deployment for observability.
func (r *DeploymentReconciler) emitEvent(ctx context.Context, dep *appsv1.Deployment, eventType, reason, msg string) {
	// controller-runtime provides an event recorder via the manager; for simplicity
	// we use direct Event object creation here.
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
	// Best-effort – ignore errors.
	_ = r.client.Create(ctx, event)
}

// labelSelectorToLabelsSelector is a utility for tests.
func labelSelectorToLabelsSelector(s *metav1.LabelSelector) (labels.Selector, error) {
	return metav1.LabelSelectorAsSelector(s)
}
