# Testing Guide — Autoscaling on `quote-app`

This guide walks through setting up the cluster from scratch and reproducing every test we ran during the RPS-based-autoscaling work.

End state after following this guide: a local Minikube cluster running `quote-app` that scales up and down based on **CPU**, **memory**, or **requests per second** — independently or combined — driven by `autoscaler-operator`.

---

## 1. What's in this project

| Component | Where | What it does |
|-----------|-------|--------------|
| `quote-app` | `quote-app/` | Spring Boot REST `/quote`, MongoDB-backed. Exposes Prometheus metrics at `/actuator/prometheus`. |
| `autoscaler-operator` | `autoscaler-operator/` | Watches Deployments labeled `autoscaler.fiit-cloud.io/enabled=true` and patches `spec.replicas` based on CPU / Mem (from Metrics Server) and RPS (from Prometheus). |
| Local cluster | `local-cluster/` | Cross-platform Makefile that installs Minikube + kubectl + Helm, starts the `fiit-cloud` cluster, and installs Prometheus stack, the operator and the demo app. |

Docker Hub images (always `:latest`):

- `xfarkasp/autoscaler-operator:latest`
- `xfarkasp/mudro-dna-be:latest`

---

## 2. Prerequisites

| Tool | Minimum |
|------|---------|
| Docker (running) | 20+ |
| `make`, `bash`, `git` | standard |
| `curl`, `python3` | for the verification commands |

The Makefile installs **Minikube**, **kubectl** and **Helm** automatically. You do **not** need Java or Maven installed — the Spring Boot app is consumed as a prebuilt image from Docker Hub.

---

## 3. Set up from scratch

```bash
git clone https://github.com/K-o-m-A/fiit-cloud.git
cd fiit-cloud/local-cluster

# Installs minikube + kubectl + helm if missing, starts the cluster,
# installs kube-prometheus-stack, autoscaler-operator and quote-app.
make all
```

This single command pulls everything from Docker Hub, so first run can take 5–10 min.

Verify:

```bash
kubectl get pods -A
helm list -A
```

Expected: pods in `apps`, `autoscaler-system`, `monitoring`, `kube-system` are `Running`; three helm releases exist (`quote-app`, `autoscaler-operator`, `prometheus-stack`).

---

## 4. Open Prometheus & Grafana

### Prometheus (PromQL playground)

```bash
nohup kubectl port-forward -n monitoring \
  svc/prometheus-stack-kube-prom-prometheus 9090:9090 >/tmp/prom-pf.log 2>&1 & disown
```

Open **http://127.0.0.1:9090**

- `/targets` — check that `serviceMonitor/apps/quote-app/0` is UP.
- `/graph` — paste PromQL queries (see Section 7).

### Grafana (pre-built dashboards)

```bash
make grafana-open    # opens browser via minikube tunnel
make grafana-creds   # prints user/password (admin / fiit-admin)
```

Useful built-in dashboards: *Kubernetes / Compute Resources / Workload* → namespace `apps`, workload `quote-app`.

---

## 5. Smoke test — everything wired correctly?

```bash
# 5a. Operator running, knows the Prometheus URL
kubectl get pods -n autoscaler-system
kubectl logs -n autoscaler-system deploy/autoscaler-operator | grep "starting autoscaler"
# expect: prometheusURL=http://prometheus-stack-kube-prom-prometheus...:9090

# 5b. Quote-app pods on the latest image
kubectl get pods -n apps -l app.kubernetes.io/name=quote-app -o jsonpath='{range .items[*]}{.metadata.name}{" → "}{.spec.containers[0].image}{"\n"}{end}'
# expect: xfarkasp/mudro-dna-be:latest

# 5c. /actuator/prometheus reachable from inside a pod
kubectl exec -n apps deploy/quote-app -- wget -qO- http://localhost:8080/actuator/prometheus 2>&1 | grep -c "^http_server_requests_seconds_count"
# expect: a number > 0

# 5d. Prometheus is scraping quote-app
curl -s "http://localhost:9090/api/v1/targets?state=active" \
  | python3 -c "import json,sys; [print(t['scrapePool'], t['health']) for t in json.load(sys.stdin)['data']['activeTargets'] if 'quote-app' in t['scrapePool']]"
# expect: serviceMonitor/apps/quote-app/0 up (one line per pod)

# 5e. RPS PromQL returns a number
curl -sG http://localhost:9090/api/v1/query \
  --data-urlencode 'query=avg(sum by (pod) (rate(http_server_requests_seconds_count{namespace="apps",pod=~"quote-app-.*"}[1m])))' \
  | python3 -c "import json,sys; d=json.load(sys.stdin); r=d['data']['result']; print('RPS =', round(float(r[0]['value'][1]),2) if r else 'NO DATA')"
```

If all five pass, the data plane is ready.

---

## 6. Three test scenarios

All three use the **same load test** (a busybox pod hitting `/quote` in a tight loop) — the only difference is which annotations are on the Deployment.

### 6.1 Test scenario A — Request-based scaling (RPS only)

This is the new behaviour added in this work.

```bash
kubectl label deployment quote-app -n apps autoscaler.fiit-cloud.io/enabled=true --overwrite

kubectl annotate deployment quote-app -n apps \
  autoscaler.fiit-cloud.io/min-replicas=1 \
  autoscaler.fiit-cloud.io/max-replicas=5 \
  autoscaler.fiit-cloud.io/scale-up-step=1 \
  autoscaler.fiit-cloud.io/scale-down-step=1 \
  autoscaler.fiit-cloud.io/scale-up-cooldown=30 \
  autoscaler.fiit-cloud.io/scale-down-cooldown=60 \
  autoscaler.fiit-cloud.io/cpu-enabled=false \
  autoscaler.fiit-cloud.io/mem-enabled=false \
  autoscaler.fiit-cloud.io/rps-enabled=true \
  autoscaler.fiit-cloud.io/rps-scale-up-threshold=10 \
  autoscaler.fiit-cloud.io/rps-scale-down-threshold=2 \
  --overwrite
```

### 6.2 Test scenario B — Resource-based scaling (CPU + Memory only)

```bash
kubectl label deployment quote-app -n apps autoscaler.fiit-cloud.io/enabled=true --overwrite

kubectl annotate deployment quote-app -n apps \
  autoscaler.fiit-cloud.io/min-replicas=1 \
  autoscaler.fiit-cloud.io/max-replicas=5 \
  autoscaler.fiit-cloud.io/scale-up-step=1 \
  autoscaler.fiit-cloud.io/scale-down-step=1 \
  autoscaler.fiit-cloud.io/scale-up-cooldown=30 \
  autoscaler.fiit-cloud.io/scale-down-cooldown=60 \
  autoscaler.fiit-cloud.io/cpu-enabled=true \
  autoscaler.fiit-cloud.io/cpu-scale-up-threshold=75 \
  autoscaler.fiit-cloud.io/cpu-scale-down-threshold=25 \
  autoscaler.fiit-cloud.io/mem-enabled=true \
  autoscaler.fiit-cloud.io/mem-scale-up-threshold=80 \
  autoscaler.fiit-cloud.io/mem-scale-down-threshold=30 \
  autoscaler.fiit-cloud.io/rps-enabled=false \
  --overwrite
```

### 6.3 Test scenario C — All three combined (production-like)

```bash
kubectl label deployment quote-app -n apps autoscaler.fiit-cloud.io/enabled=true --overwrite

kubectl annotate deployment quote-app -n apps \
  autoscaler.fiit-cloud.io/min-replicas=1 \
  autoscaler.fiit-cloud.io/max-replicas=5 \
  autoscaler.fiit-cloud.io/scale-up-step=1 \
  autoscaler.fiit-cloud.io/scale-down-step=1 \
  autoscaler.fiit-cloud.io/scale-up-cooldown=30 \
  autoscaler.fiit-cloud.io/scale-down-cooldown=60 \
  autoscaler.fiit-cloud.io/cpu-enabled=true \
  autoscaler.fiit-cloud.io/cpu-scale-up-threshold=75 \
  autoscaler.fiit-cloud.io/cpu-scale-down-threshold=25 \
  autoscaler.fiit-cloud.io/mem-enabled=true \
  autoscaler.fiit-cloud.io/mem-scale-up-threshold=80 \
  autoscaler.fiit-cloud.io/mem-scale-down-threshold=30 \
  autoscaler.fiit-cloud.io/rps-enabled=true \
  autoscaler.fiit-cloud.io/rps-scale-up-threshold=10 \
  autoscaler.fiit-cloud.io/rps-scale-down-threshold=2 \
  --overwrite
```

### 6.4 The load test (same for all scenarios)

Terminal 1 — watch replicas:

```bash
kubectl get deploy quote-app -n apps -w
```

Terminal 2 — watch operator decisions:

```bash
kubectl logs -n autoscaler-system deploy/autoscaler-operator -f \
  | grep -E "scaling decision|scaled up|scaled down"
```

Terminal 3 — generate load:

```bash
kubectl run load-gen --image=busybox:1.28 --restart=Never -n apps -- \
  /bin/sh -c "while true; do wget -q -O- http://quote-app:8080/quote; done"
```

Stop the load:

```bash
kubectl delete pod load-gen -n apps --now
```

### 6.5 What "pass" looks like

| Scenario | Expected operator behaviour |
|----------|-----------------------------|
| **A (RPS only)** | Scale up triggered by `Reason{Metric: "RPS"}` after RPS climbs past 10. Scale down triggered when RPS drops below 2. CPU/Mem never mentioned in decisions. |
| **B (CPU+Mem)** | Scale up triggered by `Reason{Metric: "CPU"}` (CPU > 75% of request). Scale down only when CPU < 25% **and** Mem < 30% (unanimous-vote rule). RPS never mentioned. |
| **C (all three)** | Any of the three metrics over its scale-up threshold triggers scale up (whichever crosses first). Scale down requires **all three** below their down thresholds — the most conservative behaviour. |

Across all three scenarios the operator must:

- Respect `min-replicas` and `max-replicas`.
- Respect the cooldown windows.
- Eventually return to `min-replicas=1` after load stops.

---

## 7. Useful Prometheus queries (paste in `/graph`)

| Purpose | Query |
|---------|-------|
| The value the operator decides on (avg RPS per pod) | `avg(sum by (pod) (rate(http_server_requests_seconds_count{namespace="apps",pod=~"quote-app-.*"}[1m])))` |
| Per-pod RPS (one line per replica) | `sum by (pod) (rate(http_server_requests_seconds_count{namespace="apps",pod=~"quote-app-.*"}[1m]))` |
| Total RPS across pods | `sum(rate(http_server_requests_seconds_count{namespace="apps",pod=~"quote-app-.*"}[1m]))` |
| Replica count over time | `kube_deployment_status_replicas{namespace="apps",deployment="quote-app"}` |
| Per-pod RPS by URI (separate probe vs real traffic) | `sum by (pod, uri) (rate(http_server_requests_seconds_count{namespace="apps",pod=~"quote-app-.*"}[1m]))` |
| CPU usage % vs request | `sum by (pod) (rate(container_cpu_usage_seconds_total{namespace="apps",pod=~"quote-app-.*",container!="POD",container!=""}[1m])) / sum by (pod) (kube_pod_container_resource_requests{namespace="apps",pod=~"quote-app-.*",resource="cpu"}) * 100` |
| Memory usage % vs request | `sum by (pod) (container_memory_working_set_bytes{namespace="apps",pod=~"quote-app-.*",container!="POD",container!=""}) / sum by (pod) (kube_pod_container_resource_requests{namespace="apps",pod=~"quote-app-.*",resource="memory"}) * 100` |

**Recommended demo panel** (Grafana or Prometheus Graph tab) — show two queries on the same chart:

1. `avg(sum by (pod) (rate(http_server_requests_seconds_count{namespace="apps",pod=~"quote-app-.*"}[1m])))`
2. `kube_deployment_status_replicas{namespace="apps",deployment="quote-app"}`

Add horizontal threshold lines at 10 (scale-up) and 2 (scale-down). The two lines tell the whole story.

---

## 8. Annotation reference

All keys are prefixed with `autoscaler.fiit-cloud.io/`.

| Key | Default | Meaning |
|-----|---------|---------|
| label `enabled` | — | Required `"true"` to opt the Deployment in. |
| `min-replicas` | `1` | Lower bound on replica count. |
| `max-replicas` | **required** | Upper bound. |
| `scale-up-step` | `1` | Replicas added per scale-up event. |
| `scale-down-step` | `1` | Replicas removed per scale-down event. |
| `scale-up-cooldown` | `60` (s) | Minimum seconds between consecutive scale-ups. |
| `scale-down-cooldown` | `300` (s) | Minimum seconds between consecutive scale-downs. |
| `cpu-enabled` | `true` | Include CPU in decisions. |
| `cpu-scale-up-threshold` | `80` (%) | Scale up when avg CPU % of requests ≥ this. |
| `cpu-scale-down-threshold` | `20` (%) | Scale down when avg CPU % of requests < this. |
| `mem-enabled` | `true` | Include memory in decisions. |
| `mem-scale-up-threshold` | `80` (%) | Same logic, memory. |
| `mem-scale-down-threshold` | `20` (%) | Same logic, memory. |
| `rps-enabled` | `false` | Include requests-per-second in decisions (queries Prometheus). |
| `rps-scale-up-threshold` | `100` (req/s per pod) | Scale up when avg per-pod RPS ≥ this. |
| `rps-scale-down-threshold` | `10` (req/s per pod) | Scale down when avg per-pod RPS < this. |

### Scaling rules (summary)

- **Scale UP**: any active metric ≥ its scale-up threshold → scale up by `scale-up-step` (capped at `max-replicas`), unless within the up-cooldown.
- **Scale DOWN**: **all** active metrics must be below their scale-down thresholds (unanimous-vote rule) → scale down by `scale-down-step` (floored at `min-replicas`), unless within the down-cooldown.
- **Hold**: anything else (mixed signals, missing metrics, cooldowns active, already at bound).
- A metric is "active" only if its `*-enabled` annotation is `true` **and** the data is available (`-1` sentinel from the collector means inactive that cycle).

---

## 9. Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `invalid autoscaler configuration; skipping` in operator log | `max-replicas` not set, or `scaleDown ≥ scaleUp` for any enabled metric | Re-annotate with valid values; operator will pick up immediately. |
| Target `apps/quote-app` not visible in Prometheus `/targets` | ServiceMonitor missing or pod uses old image (no `/actuator/prometheus`) | `kubectl get servicemonitor -n apps` should show `quote-app`. Confirm pod image is `xfarkasp/mudro-dna-be:latest`. |
| Target visible but DOWN | App not reachable on `:8080/actuator/prometheus` | `kubectl exec deploy/quote-app -n apps -- wget -qO- http://localhost:8080/actuator/prometheus` |
| Operator log: `prometheus RPS query failed; treating as inactive` | Prometheus URL wrong or unreachable from inside the cluster | Check `--prometheus-url` arg matches `kubectl get svc -n monitoring`. |
| Scale-down never fires | One active metric still above its down-threshold (unanimous-vote rule) | Disable that metric (set `*-enabled=false`) **or** wait longer **or** lower thresholds. |
| Replica oscillation (scale-up immediately followed by scale-down) | Cooldown windows too short relative to load volatility | Increase `scale-down-cooldown` first; rarely `scale-up-cooldown`. |
| Three persistent red targets in Prometheus `/targets` (`kube-etcd:2381`, `kube-controller-manager:10257`, `kube-scheduler:10259`) | Pre-existing Minikube quirk (control-plane components bind to 127.0.0.1) | Ignore — unrelated to this work. To silence, set `kubeEtcd.enabled=false` etc. in the prometheus-stack chart values. |

---

## 10. Cleanup

```bash
# Remove load generator if still around
kubectl delete pod load-gen -n apps --ignore-not-found

# Kill the Prometheus port-forward
pkill -f "kubectl port-forward.*prometheus"

# Uninstall helm releases and stop the cluster (preserves data)
cd local-cluster
make clean

# Or wipe everything including the cluster
make nuke
```

---

## 11. What was added in this work (changes vs. the previous commit)

### `quote-app/`
- `pom.xml` — added `spring-boot-starter-actuator` and `micrometer-registry-prometheus`.
- `src/main/resources/application.properties` — enabled `/actuator/prometheus` and tagged metrics with `application=mudro-dna-be`.
- `helm/quote-app/templates/servicemonitor.yaml` — new file, makes kube-prometheus-stack scrape the app every 15 s.
- `helm/quote-app/values.yaml` — image repository switched to `xfarkasp/mudro-dna-be`.

### `autoscaler-operator/`
- `pkg/prometheus/client.go` — new minimal HTTP client for Prometheus `/api/v1/query`.
- `pkg/metrics/collector.go` — added `AvgRPS` field; `Collect()` takes `deploymentName` and optionally queries Prometheus.
- `pkg/scaler/decision.go` — third metric branch (RPS) on top of CPU and Mem; removed leftover debug `fmt.Printf`.
- `pkg/scaler/decision_test.go` — five new tests (19/19 pass).
- `pkg/controller/labels.go` — three new annotation keys (`rps-enabled`, `rps-scale-up-threshold`, `rps-scale-down-threshold`); **renamed prefix from `autoscaler.yourorg.io` to `autoscaler.fiit-cloud.io`**.
- `pkg/controller/config.go` — parses and validates the three new annotations.
- `pkg/controller/reconciler.go` — threads `deploymentName` and RPS config through.
- `main.go` — new `--prometheus-url` flag.
- `helm/autoscaler-operator/values.yaml` — added `controller.prometheusUrl` default.
- `helm/autoscaler-operator/templates/deployment.yaml` — passes `--prometheus-url` to the operator.

### READMEs
- Root `README.md` and `autoscaler-operator/README.md` — annotation prefix renamed in all examples.
