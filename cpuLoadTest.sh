#!/usr/bin/env bash
# cpuLoadTest.sh — drive quote-app with HTTP load and verify CPU-based autoscaling.
#
# Operator scaling signal: CPU only. RPS and memory are disabled via annotations.
# Note: the operator reads CPU from metrics-server internally. This script polls
# Prometheus only for visibility, mirroring the operator's own formula
# (sum CPU usage / sum CPU requests across quote-app pods).
#
# Prerequisites:
#   - autoscaler-operator running in autoscaler-system namespace
#   - quote-app deployment running in apps namespace with the autoscaler label
#   - Prometheus reachable at $PROM_URL (default http://localhost:9090):
#       kubectl port-forward -n monitoring svc/prometheus-stack-kube-prom-prometheus 9090:9090 &
#   - python3 on PATH (used to parse Prometheus JSON)

set -euo pipefail

NS="apps"
DEPLOY="quote-app"
PROM_URL="${PROM_URL:-http://localhost:9090}"
LOG="cpu-experiment-$(date +%Y%m%d-%H%M%S).log"
LOAD_STREAMS="${LOAD_STREAMS:-16}"

exec > >(tee "$LOG") 2>&1

cleanup() {
  echo ">>> cleanup: removing load-gen pod"
  kubectl delete pod load-gen -n "$NS" --now --ignore-not-found >/dev/null 2>&1 || true
}
trap cleanup EXIT

# --- Preflight ----------------------------------------------------------------
echo "=== Preflight ==="
kubectl get deploy "$DEPLOY" -n "$NS" >/dev/null
if ! curl -sf "$PROM_URL/-/ready" >/dev/null; then
  echo "Prometheus not reachable at $PROM_URL"
  echo "  Hint: kubectl port-forward -n monitoring svc/prometheus-stack-kube-prom-prometheus 9090:9090 &"
  exit 1
fi
echo "  ok: deployment found, Prometheus reachable"

# --- Configure operator: CPU only ---------------------------------------------
echo "=== Configuring autoscaler annotations: CPU-only ==="
kubectl annotate deploy "$DEPLOY" -n "$NS" --overwrite \
  autoscaler.fiit-cloud.io/cpu-enabled=true \
  autoscaler.fiit-cloud.io/cpu-scale-up-threshold=75 \
  autoscaler.fiit-cloud.io/cpu-scale-down-threshold=25 \
  autoscaler.fiit-cloud.io/mem-enabled=false \
  autoscaler.fiit-cloud.io/rps-enabled=false \
  autoscaler.fiit-cloud.io/min-replicas=1 \
  autoscaler.fiit-cloud.io/max-replicas=5 \
  autoscaler.fiit-cloud.io/scale-up-step=1 \
  autoscaler.fiit-cloud.io/scale-down-step=1 \
  autoscaler.fiit-cloud.io/scale-up-cooldown=30 \
  autoscaler.fiit-cloud.io/scale-down-cooldown=60 >/dev/null
echo "  cpu-scale-up=75%  cpu-scale-down=25%  min=1  max=5  step=1  cooldown=30/60s"

# --- Prometheus helpers -------------------------------------------------------
CPU_QUERY='( sum(rate(container_cpu_usage_seconds_total{namespace="apps",pod=~"quote-app-.*",container="quote-app"}[1m]))
             /
             sum(kube_pod_container_resource_requests{namespace="apps",pod=~"quote-app-.*",container="quote-app",resource="cpu"})
           ) * 100'
RPS_QUERY='avg(sum by (pod) (rate(http_server_requests_seconds_count{namespace="apps",pod=~"quote-app-.*"}[1m])))'

query_prom() {
  local q="$1"
  curl -sG "$PROM_URL/api/v1/query" --data-urlencode "query=$q" \
    | python3 -c "import json,sys; d=json.load(sys.stdin); r=d['data']['result']; print(round(float(r[0]['value'][1]),1) if r else 0)" \
    2>/dev/null || echo "0"
}

snap() {
  local elapsed="$1"
  local r cpu rps
  r=$(kubectl get deploy "$DEPLOY" -n "$NS" -o jsonpath='{.status.replicas}')
  cpu=$(query_prom "$CPU_QUERY")
  rps=$(query_prom "$RPS_QUERY")
  printf "  t=%4ds  replicas=%s  CPU=%s%%  RPS=%s\n" "$elapsed" "$r" "$cpu" "$rps"
}

# --- Phase 1: baseline --------------------------------------------------------
echo "=== Phase 1: baseline (30s, no load) ==="
START=$SECONDS
while [ $((SECONDS-START)) -lt 30 ]; do
  snap "$((SECONDS-START))"
  sleep 10
done

# --- Phase 2: apply parallel load ---------------------------------------------
echo "=== Phase 2: applying $LOAD_STREAMS parallel HTTP streams ==="
kubectl delete pod load-gen -n "$NS" --now --ignore-not-found >/dev/null 2>&1 || true
kubectl run load-gen --image=busybox:1.28 --restart=Never -n "$NS" -- \
  /bin/sh -c "for i in \$(seq 1 $LOAD_STREAMS); do (while true; do wget -q -O- http://quote-app:8080/quote >/dev/null; done) & done; wait"
kubectl wait --for=condition=Ready pod/load-gen -n "$NS" --timeout=60s >/dev/null

# --- Phase 3: watch scale-up --------------------------------------------------
echo "=== Phase 3: watching scale-UP (max 180s) ==="
UP_START=$SECONDS
FIRST_SCALE_AT=""
while [ $((SECONDS-UP_START)) -lt 180 ]; do
  r=$(kubectl get deploy "$DEPLOY" -n "$NS" -o jsonpath='{.status.replicas}')
  if [ -z "$FIRST_SCALE_AT" ] && [ "$r" -gt 1 ]; then
    FIRST_SCALE_AT=$((SECONDS-UP_START))
    echo "  *** first scale-up after ${FIRST_SCALE_AT}s ***"
  fi
  snap "$((SECONDS-UP_START))"
  if [ "$r" = "5" ]; then
    echo "  -> reached max replicas, stopping scale-up watch"
    break
  fi
  sleep 10
done

# --- Phase 4: stop load -------------------------------------------------------
echo "=== Phase 4: stopping load ==="
kubectl delete pod load-gen -n "$NS" --now --ignore-not-found >/dev/null

# --- Phase 5: watch scale-down ------------------------------------------------
echo "=== Phase 5: watching scale-DOWN (max 600s) ==="
DOWN_START=$SECONDS
while [ $((SECONDS-DOWN_START)) -lt 600 ]; do
  r=$(kubectl get deploy "$DEPLOY" -n "$NS" -o jsonpath='{.status.replicas}')
  snap "$((SECONDS-DOWN_START))"
  if [ "$r" = "1" ]; then
    echo "  -> back to min replicas, done"
    break
  fi
  sleep 15
done

# --- Summary ------------------------------------------------------------------
echo "=== Summary ==="
echo "  First scale-up : ${FIRST_SCALE_AT:-N/A}s after load applied (SLO: <=60s)"
echo "  Log file       : $LOG"
echo "  Recent events  :"
kubectl get events -n "$NS" --sort-by=.lastTimestamp \
  --field-selector reason=ScaledUp,reason=ScaledDown 2>/dev/null \
  | tail -10 || true
