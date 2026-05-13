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

type DeploymentSnapshot struct {
	PodCount int

	AvgCPUUtilizationPct int32

	AvgMemUtilizationPct int32

	AvgRPS int32
}

type Collector struct {
	k8sClient     client.Client
	metricsClient versioned.Interface
	promClient    *prometheus.Client
}

func New(c client.Client, mc versioned.Interface, pc *prometheus.Client) *Collector {
	return &Collector{k8sClient: c, metricsClient: mc, promClient: pc}
}

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
