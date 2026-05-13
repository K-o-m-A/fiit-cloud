# Testing CPU + Memory autoscaling on `quote-app`

This guide is for the **CPU + Memory** test scenario (scenario B from `TESTING.md`). Unlike the RPS test, this one exercises the resource-based metrics that come from the Kubernetes Metrics Server and requires JVM-aware threshold tuning to actually scale **down**.

If you only care about the simplest demo, do the RPS scenario in `TESTING.md` instead — it's deterministic and finishes in ~5 minutes.

---

## 1. Why CPU + Memory needs special handling for JVM apps

The naive setup `cpu-scale-down-threshold=25 / mem-scale-down-threshold=20` doesn't scale **down** for `quote-app`, for two structural reasons:

| Problem | Root cause |
|---------|-----------|
| Memory % never drops below ~65–75% even at idle | The JVM holds onto whatever heap it grew into. With a `256Mi` request, idle Spring Boot already uses 170Mi → 67%. Memory never qualifies for scale-down. |
| Pods crash-loop during scale-up | The chart's `livenessProbe` had no startup delay / `startupProbe`. New pods got SIGTERM at ~30s, before Spring Boot finished booting on a busy node. |

Both are fixed in the chart now (commit ahead of this guide):

- `quote-app/helm/quote-app/values.yaml` and `templates/deployment.yaml` — added `startupProbe` with a 5-minute budget (`failureThreshold: 60`, `periodSeconds: 5`). Liveness/readiness now hit `/actuator/health`.
- The autoscaler thresholds need to be set with JVM heap behaviour in mind (see Section 3).

---

## 2. Prerequisites

You've already done these in `TESTING.md`:

- `make all` from `local-cluster/`
- Prometheus port-forward on `localhost:9090`
- Quote-app on image `xfarkasp/mudro-dna-be:latest` (which contains the `startupProbe` fix)
- Operator on image `xfarkasp/autoscaler-operator:latest`

Confirm the chart has the probe fix:

```bash
kubectl get deploy quote-app -n apps -o jsonpath='{.spec.template.spec.containers[0].startupProbe}' | python3 -m json.tool
# expect: { "httpGet": { "path": "/actuator/health", ... }, "failureThreshold": 60, "periodSeconds": 5 }
```

If empty, your chart predates the fix — pull the latest commit and `helm upgrade quote-app helm/quote-app -n apps --reset-values --wait` first.

---

## 3. Apply the CPU + Memory annotations

The key trick: **shift memory thresholds high enough that idle JVM heap doesn't block scale-down**. CPU becomes the primary signal; memory is a guardrail for actual memory pressure.

```bash
kubectl label deployment quote-app -n apps \
  autoscaler.fiit-cloud.io/enabled=true --overwrite

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
  autoscaler.fiit-cloud.io/mem-scale-up-threshold=95 \
  autoscaler.fiit-cloud.io/mem-scale-down-threshold=90 \
  autoscaler.fiit-cloud.io/rps-enabled=false \
  --overwrite
```

### Why these specific numbers

| Annotation | Value | Reason |
|------------|-------|--------|
| `cpu-scale-up-threshold` | 75 | Standard HPA-style default; matches operator's other examples. |
| `cpu-scale-down-threshold` | 25 | CPU drops fast to near 0 when load stops, so 25 is easily reached. |
| `mem-scale-up-threshold` | **95** | Only treat memory as a scale-up signal in extreme cases; default 80 would always be tripped by JVM baseline. |
| `mem-scale-down-threshold` | **90** | JVM heap baseline is 65–75%; using 90 keeps memory voting "scale down" almost always, so it doesn't block CPU's vote. |
| `rps-enabled` | false | Cleanly disable the RPS path so this is a pure resource-based test. |
| `scale-up-cooldown` | 30 | Tightened from default 60 for a watchable demo. |
| `scale-down-cooldown` | 60 | Tightened from default 300 for a watchable demo. |

---

## 4. The test — three terminals

### Terminal 1 — watch replicas live

```bash
kubectl get deploy quote-app -n apps -w
```

### Terminal 2 — watch operator decisions

```bash
kubectl logs -n autoscaler-system deploy/autoscaler-operator -f \
  | grep -E "scaling decision|scaled up|scaled down"
```

### Terminal 3 — drive the load

```bash
kubectl run load-gen --image=busybox:1.28 --restart=Never -n apps -- \
  /bin/sh -c "while true; do wget -q -O- http://quote-app:8080/quote; done"
```

When replicas reach 5, stop the load:

```bash
kubectl delete pod load-gen -n apps --now
```

---

## 5. What "pass" looks like

### Scale-up phase (~1-2 minutes)

| Time after load start | Replicas | Operator decision |
|------|----------|------------------|
| ~0–15s | 1 (or current) | `Hold` |
| ~15-30s | 1 → 2 | `ScaleUp [CPU observed=80%+ threshold=>=75%]` |
| ~30-60s | 2 → 3 | `ScaleUp` (cooldown expired) |
| ~60-90s | 3 → 4 → 5 | further `ScaleUp` events |
| sustained | stays at 5 | `Hold [... guard observed=at max replicas]` |

Memory usually stays in the 70-85% range during load — under your `mem-scale-up-threshold=95`, so it doesn't double-trigger. CPU is the only driver.

### Scale-down phase (~2-5 minutes after load stops)

| Time after load stop | Replicas | What's happening |
|------|----------|------------------|
| ~0-30s | 5 | CPU dropping but not yet under 25% |
| ~30-60s | 5 | CPU now under 25%, memory ~65-80% (under 90), but scale-down cooldown |
| ~60-120s | 5 → 4 | First `ScaleDown [all metrics below threshold]` |
| ~120-180s | 4 → 3 | Second scale-down (cooldown elapsed) |
| ~180-240s | 3 → 2 → 1 | Drops to `min-replicas=1` |

Total demo time: **~5-7 minutes** vs ~5 minutes for the RPS demo.

---

## 6. Pass criteria

- Replicas grow above 1 during load (1 → 5).
- Each scale step respects the **30s up / 60s down cooldown** (no faster).
- Capped at **5** (max-replicas).
- After load stops, replicas eventually return to **1** (min-replicas).
- No pods enter `CrashLoopBackOff` during scale-up.
- Operator log shows **CPU** (sometimes Memory) as the triggering metric — never RPS.

---

## 7. If something goes wrong

| Symptom | Cause | Fix |
|---------|-------|-----|
| Pods crash-loop during scale-up | Probe fix missing in this deployment | `helm upgrade quote-app helm/quote-app -n apps --reset-values --wait` |
| Scale-up never fires | CPU isn't reaching 75% — load too gentle, or replicas haven't started receiving traffic yet | Wait longer; the busybox loop ramps up over 30s. Check `kubectl top pods` for actual CPU. |
| Scale-down stuck at 5 with memory ~67% | Memory was somehow re-enabled with old threshold | Confirm `mem-scale-down-threshold=90` is on the deployment (`kubectl describe deploy quote-app -n apps \| grep mem-scale-down`) |
| Scale-down stuck with CPU still elevated | Spring Boot's JVM doing background GC / migration work after load | Wait another 60s; if CPU stays >25%, lower `cpu-scale-down-threshold=15` |
| Operator log shows `invalid autoscaler configuration` | Typo in annotation key, or `scaleDown ≥ scaleUp` somewhere | Re-run the annotate block from Section 3. |

---

## 8. The point of this scenario

CPU+Memory autoscaling for JVM applications is **realistic but constrained**:

- **CPU** is a usable signal — it rises with load, drops with idle.
- **Memory** is mostly a *guardrail* — JVMs hold heap, so it's bad for triggering either direction in the steady state. The conservative "all metrics must agree" scale-down rule combined with sticky JVM memory is exactly why HPA documentation recommends against memory-based scale-down for JVM workloads.

If you walk away with one takeaway: **for HTTP services, prefer RPS-based scaling** (`TESTING.md` scenario A). Use CPU+Mem for *non-HTTP* workloads where there's no useful application-level signal.

---

## 9. Cleanup (return to known-good state)

```bash
# stop load if still running
kubectl delete pod load-gen -n apps --ignore-not-found

# remove the test annotations entirely (operator stops watching)
kubectl label deployment quote-app -n apps autoscaler.fiit-cloud.io/enabled-
for k in min-replicas max-replicas scale-up-step scale-down-step \
         scale-up-cooldown scale-down-cooldown \
         cpu-enabled cpu-scale-up-threshold cpu-scale-down-threshold \
         mem-enabled mem-scale-up-threshold mem-scale-down-threshold \
         rps-enabled; do
  kubectl annotate deployment quote-app -n apps "autoscaler.fiit-cloud.io/$k-" 2>/dev/null
done

# scale manually back to 1 if needed
kubectl scale deploy quote-app -n apps --replicas=1
```
