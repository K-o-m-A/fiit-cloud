package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	// TotalRPS is the current total request rate (req/s) for the whole Deployment.
	// -1 when RPS collection is disabled or Prometheus is unavailable.
	TotalRPS float64
}

// RPSPerPod returns TotalRPS divided by PodCount (or -1 if unavailable).
func (s *DeploymentSnapshot) RPSPerPod() float64 {
	if s.TotalRPS < 0 || s.PodCount == 0 {
		return -1
	}
	return s.TotalRPS / float64(s.PodCount)
}

// Collector fetches resource metrics from the Kubernetes Metrics Server and
// request-rate metrics from Prometheus.
type Collector struct {
	k8sClient     client.Client
	prometheusURL string
	httpClient    *http.Client
}

// New creates a ready-to-use Collector.
func New(c client.Client, prometheusURL string) *Collector {
	return &Collector{
		k8sClient:     c,
		prometheusURL: prometheusURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Collect returns a DeploymentSnapshot for all running pods matching selector.
// rpsEnabled / rpsQuery control whether a Prometheus query is issued.
func (c *Collector) Collect(
	ctx context.Context,
	namespace string,
	selector labels.Selector,
	rpsEnabled bool,
	rpsQuery string, // empty = use built-in query
	deploymentName string,
) (*DeploymentSnapshot, error) {

	snap := &DeploymentSnapshot{
		AvgCPUUtilizationPct: -1,
		AvgMemUtilizationPct: -1,
		TotalRPS:             -1,
	}

	// --- collect CPU / Memory via Metrics Server ---
	if err := c.collectResourceMetrics(ctx, namespace, selector, snap); err != nil {
		return snap, fmt.Errorf("resource metrics: %w", err)
	}

	// --- collect RPS via Prometheus ---
	if rpsEnabled {
		q := rpsQuery
		if q == "" {
			q = defaultRPSQuery(namespace, deploymentName)
		}
		rps, err := c.queryPrometheusScalar(ctx, q)
		if err != nil {
			// Non-fatal: log and continue without RPS-based scaling.
			return snap, fmt.Errorf("prometheus rps query: %w", err)
		}
		snap.TotalRPS = rps
	}

	return snap, nil
}

// defaultRPSQuery builds a PromQL expression that counts HTTP requests per second
// for a Deployment. Adjust the metric name / labels to match your instrumentation.
func defaultRPSQuery(namespace, deployment string) string {
	return fmt.Sprintf(
		`sum(rate(http_requests_total{namespace="%s",deployment="%s"}[2m]))`,
		namespace, deployment,
	)
}

// collectResourceMetrics populates CPU/Mem fields in the snapshot by iterating
// over all running pods and fetching their PodMetrics objects.
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

		pm := &metricsv1beta1.PodMetrics{}
		if err := c.k8sClient.Get(ctx,
			types.NamespacedName{Namespace: namespace, Name: pod.Name},
			pm,
		); err != nil {
			// Pod metrics not yet available (e.g. just started). Skip.
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

// --- Prometheus instant query ---

type promResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Value [2]interface{} `json:"value"` // [unixTimestamp, "value"]
		} `json:"result"`
	} `json:"data"`
}

// queryPrometheusScalar executes a PromQL instant query and returns the scalar result.
func (c *Collector) queryPrometheusScalar(ctx context.Context, query string) (float64, error) {
	u, err := url.Parse(c.prometheusURL + "/api/v1/query")
	if err != nil {
		return 0, fmt.Errorf("invalid prometheus URL: %w", err)
	}
	q := u.Query()
	q.Set("query", query)
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return 0, fmt.Errorf("building request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("prometheus returned %d: %s", resp.StatusCode, body)
	}

	var pr promResponse
	if err := json.Unmarshal(body, &pr); err != nil {
		return 0, fmt.Errorf("parsing response: %w", err)
	}
	if pr.Status != "success" {
		return 0, fmt.Errorf("prometheus status: %s", pr.Status)
	}
	if len(pr.Data.Result) == 0 {
		// No time-series means zero traffic.
		return 0, nil
	}

	rawVal, ok := pr.Data.Result[0].Value[1].(string)
	if !ok {
		return 0, fmt.Errorf("unexpected value type in prometheus response")
	}
	val, err := strconv.ParseFloat(rawVal, 64)
	if err != nil {
		return 0, fmt.Errorf("parsing float from prometheus: %w", err)
	}

	return val, nil
}
