# autoscaler-operator

A lightweight Kubernetes operator written in Go that automatically scales
labeled Deployments based on **CPU utilisation** and **memory utilisation**.

## How it works

```
Deployment (labeled)
       │
       ▼
  Reconciler  ──── Kubernetes Metrics Server ──► CPU / Memory %
       │
       ▼
  Scaler (pure logic, fully tested)
       │
       ├── ScaleUp?   → PATCH deployment.spec.replicas += step
       ├── ScaleDown? → PATCH deployment.spec.replicas -= step
       └── Hold?      → requeue after 30 s
```

The operator uses **controller-runtime** (kubebuilder-style) for its
reconciliation loop and informer/cache machinery.

---

## Opt-in a Deployment

Add a single label to opt in, then tune with annotations:

```yaml
metadata:
  labels:
    autoscaler.yourorg.io/enabled: "true"   # ← opt-in

  annotations:
    autoscaler.yourorg.io/min-replicas: "2"
    autoscaler.yourorg.io/max-replicas: "20"

    # CPU (% of requests)
    autoscaler.yourorg.io/cpu-scale-up-threshold:   "75"
    autoscaler.yourorg.io/cpu-scale-down-threshold: "25"

    # Memory (% of requests)
    autoscaler.yourorg.io/mem-scale-up-threshold:   "80"
    autoscaler.yourorg.io/mem-scale-down-threshold: "30"

```

Full annotation reference: [`pkg/controller/labels.go`](pkg/controller/labels.go)

---

## Scaling rules

| Condition | Action |
|-----------|--------|
| **Any** active metric exceeds its scale-up threshold | Scale **up** by `scale-up-step` |
| **All** active metrics are below their scale-down threshold | Scale **down** by `scale-down-step` |
| Mixed signals (some up, some down) | **Hold** |
| Already at `max-replicas` | **Hold** |
| Already at `min-replicas` | **Hold** |
| Within cooldown window | **Hold** |

> **Important:** resource `requests` must be set on containers for CPU/Memory
> utilisation percentages to be meaningful.

---

## Prerequisites

| Component | Purpose |
|-----------|---------|
| Kubernetes Metrics Server | CPU & Memory data |

---

## Quick start

```bash
# 1. Build the image
make docker-build docker-push IMAGE=yourorg/autoscaler-operator TAG=v1.0.0

# 2. Deploy the operator
make deploy

# 3. Apply the example deployment
make example

# 4. Watch logs
make logs
```

---

## Operator flags

| Flag | Default | Description |
|------|---------|-------------|
| `--watch-namespace` | `""` (all) | Restrict to a single namespace |
| `--sync-period` | `30s` | How often the controller re-evaluates |
| `--leader-elect` | `false` | Enable HA leader election |
| `--metrics-bind-address` | `:8080` | Controller-runtime metrics endpoint |

---

