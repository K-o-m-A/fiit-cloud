echo "=== Phase 1: baseline (5s) ==="
  sleep 5
  kubectl get deploy quote-app -n apps -o jsonpath='replicas={.status.replicas}{"\n"}'

  echo "=== Phase 2: applying load ==="
  kubectl run load-gen --image=busybox:1.28 --restart=Never -n apps -- \
    /bin/sh -c "while true; do wget -q -O- http://quote-app:8080/quote; done"

  echo "=== Phase 3: watching scale-UP ==="
  START=$SECONDS
  while [ $((SECONDS-START)) -lt 180 ]; do
    R=$(kubectl get deploy quote-app -n apps -o jsonpath='{.status.replicas}')
    RPS=$(curl -sG http://localhost:9090/api/v1/query \
          --data-urlencode 'query=avg(sum by (pod) (rate(http_server_requests_seconds_count{namespace="apps",pod=~"quote-app-.*"}[1m])))' \
          | python3 -c "import json,sys; d=json.load(sys.stdin); r=d['data']['result']; print(round(float(r[0]['value'][1]),1) if r else 0)")
    printf "  t=%3ds replicas=%s RPS=%s\n" "$((SECONDS-START))" "$R" "$RPS"
    [ "$R" = "5" ] && echo "  → reached max, breaking" && break
    sleep 10
  done

  echo "=== Phase 4: stopping load ==="
  kubectl delete pod load-gen -n apps --now

  echo "=== Phase 5: watching scale-DOWN ==="
  START=$SECONDS
  while [ $((SECONDS-START)) -lt 360 ]; do
    R=$(kubectl get deploy quote-app -n apps -o jsonpath='{.status.replicas}')
    RPS=$(curl -sG http://localhost:9090/api/v1/query \
          --data-urlencode 'query=avg(sum by (pod) (rate(http_server_requests_seconds_count{namespace="apps",pod=~"quote-app-.*"}[1m])))' \
          | python3 -c "import json,sys; d=json.load(sys.stdin); r=d['data']['result']; print(round(float(r[0]['value'][1]),1) if r else 0)")
    printf "  t=%3ds replicas=%s RPS=%s\n" "$((SECONDS-START))" "$R" "$RPS"
    [ "$R" = "1" ] && echo "  → back to min, done" && break
    sleep 10
  done