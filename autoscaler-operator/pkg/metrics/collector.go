package metrics

import (
	"context"
	"fmt"

	"github.com/K-o-m-A/fiit-cloud/autoscaler-operator/pkg/prometheus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	versioned "k8s.io/metrics/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
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

	// AvgRPS is the mean requests-per-second per pod, queried from Prometheus.
	// -1 when Prometheus is not configured or the query failed.
	AvgRPS int32
}

// Collector fetches resource metrics from the Kubernetes Metrics Server and,
// optionally, request-rate metrics from Prometheus.
type Collector struct {
	k8sClient     client.Client
	metricsClient versioned.Interface
	promClient    *prometheus.Client // nil if RPS scraping is not configured
}

// New returns a Collector. promClient may be nil; in that case AvgRPS is always -1.
func New(c client.Client, mc versioned.Interface, pc *prometheus.Client) *Collector {
	return &Collector{k8sClient: c, metricsClient: mc, promClient: pc}
}

// Collect returns a DeploymentSnapshot for all running pods matching selector.
// deploymentName is used to build the per-pod RPS PromQL query.
func (c *Collector) Collect(
	ctx context.Context,
	namespace string,
	deploymentName string,
	selector labels.Selector,
) (*DeploymentSnapshot, error) {

	snap := &DeploymentSnapshot{
		AvgCPUUtilizationPct: -1,
		AvgMemUtilizationPct: -1,
		AvgRPS:               -1,
	}

	resourceErr := c.collectResourceMetrics(ctx, namespace, selector, snap)

	// PodCount must reflect running pods even when metrics-server is unavailable,
	// so RPS-only scaling can proceed.
	if snap.PodCount == 0 {
		if n, err := c.countRunningPods(ctx, namespace, selector); err == nil {
			snap.PodCount = n
		}
	}

	c.collectRPS(ctx, namespace, deploymentName, snap)

	if resourceErr != nil {
		return snap, fmt.Errorf("resource metrics: %w", resourceErr)
	}
	return snap, nil
}

// countRunningPods returns the number of Running pods matching selector.
// Used as a fallback when metrics-server is not installed so that RPS-only
// scaling can still make decisions.
func (c *Collector) countRunningPods(
	ctx context.Context,
	namespace string,
	selector labels.Selector,
) (int, error) {
	podList := &corev1.PodList{}
	if err := c.k8sClient.List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return 0, err
	}
	n := 0
	for i := range podList.Items {
		if podList.Items[i].Status.Phase == corev1.PodRunning {
			n++
		}
	}
	return n, nil
}

// collectResourceMetrics populates CPU/Mem fields in the snapshot by iterating
// over all running pods and fetching their PodMetrics via a List call.
func (c *Collector) collectResourceMetrics(
	ctx context.Context,
	namespace string,
	selector labels.Selector,
	snap *DeploymentSnapshot,
) error {

	podList := &corev1.PodList{}
	if err := c.k8sClient.List(ctx, podList,
		client.InNamespace(namespace),
		client.MatchingLabelsSelector{Selector: selector},
	); err != nil {
		return fmt.Errorf("listing pods: %w", err)
	}

	pmList, err := c.metricsClient.MetricsV1beta1().PodMetricses(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("listing pod metrics: %w", err)
	}

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
			continue
		}

		for _, cm := range pm.Containers {
			totalCPUMilli += cm.Usage.Cpu().MilliValue()
			totalMemBytes += cm.Usage.Memory().Value()
		}

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

// collectRPS asks Prometheus for the per-pod request rate of this deployment,
// averaged across pods. Leaves AvgRPS at -1 if Prometheus is not configured or
// the query fails (the scaler treats -1 as "metric inactive").
func (c *Collector) collectRPS(
	ctx context.Context,
	namespace, deploymentName string,
	snap *DeploymentSnapshot,
) {
	if c.promClient == nil {
		return
	}
	logger := log.FromContext(ctx)

	query := fmt.Sprintf(
		`avg(sum by (pod) (rate(http_server_requests_seconds_count{namespace=%q,pod=~%q}[1m])))`,
		namespace, deploymentName+"-.*",
	)
	val, err := c.promClient.Query(ctx, query)
	if err != nil {
		logger.Error(err, "prometheus RPS query failed; treating as inactive")
		return
	}
	snap.AvgRPS = int32(val)
}
