package metrics

import (
	"context"
	"fmt"
	"sigs.k8s.io/controller-runtime/pkg/log" 

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    versioned "k8s.io/metrics/pkg/client/clientset/versioned"
)

// DeploymentSnapshot is the aggregated metric reading for a single reconcile cycle.
type DeploymentSnapshot struct {
	// Number of running pods metrics were collected from.
	PodCount int

	// AvgCPUUtilizationPct is the mean CPU utilisation across pods (% of requests).
	// -1 when no CPU requests are set.
	AvgCPUUtilizationPct int32

	// AvgMemUtilizationPct is the mean memory utilisation across pods (% of requests).
	// -1 when no memory requests are set.
	AvgMemUtilizationPct int32
}

// Collector fetches resource metrics from the Kubernetes Metrics Server.
type Collector struct {
	k8sClient client.Client
	metricsClient versioned.Interface
}

func New(c client.Client, mc versioned.Interface) *Collector {
    return &Collector{k8sClient: c, metricsClient: mc}
}

// Collect returns a DeploymentSnapshot for all running pods matching selector.
func (c *Collector) Collect(
	ctx context.Context,
	namespace string,
	selector labels.Selector,
) (*DeploymentSnapshot, error) {

	snap := &DeploymentSnapshot{
		AvgCPUUtilizationPct: -1,
		AvgMemUtilizationPct: -1,
	}

	if err := c.collectResourceMetrics(ctx, namespace, selector, snap); err != nil {
		return snap, fmt.Errorf("resource metrics: %w", err)
	}

	return snap, nil
}

// collectResourceMetrics populates CPU/Mem fields in the snapshot by iterating
// over all running pods and fetching their PodMetrics via a List call (avoids
// the watch cache which metrics.k8s.io does not support).
func (c *Collector) collectResourceMetrics(
	ctx context.Context,
	namespace string,
	selector labels.Selector,
	snap *DeploymentSnapshot,
) error {

	// List running pods matching the deployment selector.
	podList := &corev1.PodList{}
	if err := c.k8sClient.List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

	logger := log.FromContext(ctx)
    logger.Info("DEBUG pods found", "count", len(podList.Items))
    for _, p := range podList.Items {
        logger.Info("DEBUG pod", "name", p.Name, "phase", p.Status.Phase)
    }

	pmList, err := c.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing pod metrics: %w", err)
	}

	logger.Info("DEBUG pod metrics found", "count", len(pmList.Items))
    for _, pm := range pmList.Items {
        logger.Info("DEBUG podmetric", "name", pm.Name)
    }

	// Build a name → PodMetrics lookup map for O(1) access.
	pmMap := make(map[string]metricsv1beta1.PodMetrics, len(pmList.Items))
	for _, pm := range pmList.Items {
		pmMap[pm.Name] = pm
	}

	var (
		totalCPUMilli int64
		totalMemBytes int64
		totalCPUReq   int64
		totalMemReq   int64
		validPods     int
	)

	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}

		pm, ok := pmMap[pod.Name]
		if !ok {
			// Metrics not yet available for this pod (e.g. just started). Skip.
			continue
		}

		// Sum usage across all containers in this pod.
		for _, cm := range pm.Containers {
			totalCPUMilli += cm.Usage.Cpu().MilliValue()
			totalMemBytes += cm.Usage.Memory().Value()
		}

		// Sum resource requests (needed for % utilisation calculation).
		for _, ctr := range pod.Spec.Containers {
			if q, ok := ctr.Resources.Requests[corev1.ResourceCPU]; ok {
				totalCPUReq += q.MilliValue()
			}
			if q, ok := ctr.Resources.Requests[corev1.ResourceMemory]; ok {
				totalMemReq += q.Value()
			}
		}

		validPods++
	}

	snap.PodCount = validPods
	if validPods == 0 {
		return nil
	}

	if totalCPUReq > 0 {
		snap.AvgCPUUtilizationPct = int32((totalCPUMilli * 100) / totalCPUReq)
	}
	if totalMemReq > 0 {
		snap.AvgMemUtilizationPct = int32((totalMemBytes * 100) / totalMemReq)
	}

	return nil
}