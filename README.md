# FIIT-CLOUD-PROJECT
### Authors: Peter Farkaš, Darius-Dušan Horvath, Adrián Komanek, Frederik Duvač

## OpenStack Terraform

Terraform scripts in `openstack/terraform` provision a minimal Kubernetes environment in OpenStack:
- one control-plane VM
- one worker VM
- private network, subnet, router, and security group
- floating IP for control-plane access
- bootstrap scripts to initialize control plane, join worker, and fetch kubeconfig


### What Gets Provisioned
The Terraform flow creates Kubernetes-ready infrastructure in this order:
1. Networking layer (network, subnet, router + router interface)
2. Security group with Kubernetes-related rules (API server, node communication, NodePort range, etc.)
3. Ports for control plane and worker nodes
4. Two instances: one control plane and one worker
5. Floating IP association for the control plane
6. Remote bootstrap via templates:
	- control plane is initialized with `kubeadm init`
	- worker joins the cluster via generated join token
	- kubeconfig is fetched locally for kubectl access

### OpenStack Structure
- `openstack/terraform/main.tf` - infrastructure resources and bootstrap steps
- `openstack/terraform/variables.tf` - input variables
- `openstack/terraform/terraform.tfvars` - environment values
- `openstack/terraform/outputs.tf` - outputs (IPs, SSH command)
- `openstack/terraform/templates/` - shell templates used by provisioners

### OpenStack Quick Start
From `openstack/terraform`:

```bash
terraform init
terraform plan
terraform apply
```

After apply, kubeconfig is saved to `~/.kube/openstack-k8s.conf`.

Use it with:

```bash
export KUBECONFIG=~/.kube/openstack-k8s.conf
kubectl get nodes
```

To destroy all created resources:

```bash
terraform destroy
```

Due to slow disk I/O on the SAV OpenStack environment causing kubeadm initialization failures, no cluster is currently running there. As a result, we are using a local cluster instance for operator testing and development.


## Local Cluster Makefile

`local-cluster/Makefile` provides local Kubernetes setup with Minikube and Prometheus stack. Our scripts supports Windows/MAC/Linux operating systems.

### What It Does
- installs `minikube`, `kubectl`, and `helm`
- creates Minikube cluster `fiit-cloud` (4 CPU, 4096 MB RAM)
- installs `kube-prometheus-stack` in namespace `monitoring`
- provides helper targets for status, Grafana access, and cleanup

### Common Commands
Run from `local-cluster`:

```bash
make help
make install
make start-cluster
make install-prometheus-stack
make status
make clean
```

One-shot setup:

```bash
make all
```
#  Quote App — Backend

A Spring Boot REST API that serves random quotes from MongoDB. Fully containerized with Docker and deployable to Kubernetes via Helm.

---

## Project Structure

```
qoute-app/
├── Dockerfile                  # Multi-stage Docker build
├── docker-compose.yaml         # Local development (app + MongoDB)
├── pom.xml
├── helm/
│   └── quote-app/              # Helm chart for Kubernetes deployment
│       ├── Chart.yaml          # Chart metadata + MongoDB dependency
│       ├── values.yaml         # Configuration values
│       └── templates/
│           ├── deployment.yaml
│           ├── service.yaml
│           └── ...
└── src/
    └── main/
        ├── java/compose/project/mudrodnabe/
        │   ├── MudroDnaBeApplication.java
        │   ├── entities/
        │   │   └── Quote.java
        │   └── migrations/
        │       └── SeedQuotesMigration.java
        └── resources/
            └── application.properties
```

---

## Quick Start

### Prerequisites
- Docker Desktop (with Kubernetes enabled)
- Java 21
- Maven
- `kubectl`
- `helm`

---

## Local Development (Docker Compose)

Run the app and MongoDB locally with Docker Compose:

```bash
# Start MongoDB
docker-compose up -d

# Run the Spring Boot app locally
./mvnw spring-boot:run
```

The API will be available at `http://localhost:8080`.

---

## Docker

### Build the image

```bash
docker build -t yourusername/quote-app:latest .
```

### Run the container

```bash
docker run --env-file .env -p 8080:8080 yourusername/quote-app:latest
```

### Push to Docker Hub

```bash
docker push yourusername/quote-app:latest
```

---

## Kubernetes (Helm)

The Helm chart deploys both the app and MongoDB together using a Bitnami MongoDB dependency.

### Using the Makefile (recommended)

From the `fiit-cloud/local-cluster/` directory:

```bash
# Deploy everything (cluster + monitoring + quote app)
make all

# Deploy only the quote app
make install-quote-app

# Uninstall the quote app
make uninstall-quote-app

# Port forward to test locally
make port-forward-quote-app
```

---

## API Endpoints

| Method | Endpoint | Description          |
|--------|----------|----------------------|
| GET    | `/quote` | Returns a random quote |

# Autoscaler Operator

`autoscaler-operator` is a lightweight Kubernetes operator written in Go that scales labeled Deployments automatically based on (for now) CPU and memory utilisation. In future the loadbalancing using amount of incoming requests will be implemented.

### What It Does
- watches Deployments based on label
- reads CPU and memory metrics from Kubernetes Metrics Server
- decides whether to scale up, scale down, or hold
- updates `deployment.spec.replicas` through the controller reconciliation loop

### How It Works
1. A Deployment is marked for autoscaling with `autoscaler.yourorg.io/enabled: "true"`
2. The operator reads the current CPU and memory utilisation for that workload
3. The scaler applies the configured thresholds and replica limits
4. The reconciler patches the Deployment when scaling is needed
5. If no change is needed, the operator requeues and checks again later

### Decision Making (When to Scale Up/Down)
The operator follows strict rules in each reconciliation cycle:

1. Scale Up rule:
	- if any of the metrics (CPU or memory) is greater than or equal to its scale-up threshold, the operator requests scale up
	- replicas are increased by `scale-up-step` and capped at `max-replicas`

2. Scale Down rule:
	- scale down happens only if all active metrics are below their scale-down thresholds
	- replicas are decreased by `scale-down-step` and never go below `min-replicas`

3. Hold rule:
	- if metrics are mixed (for example CPU says scale up but memory does not), the operator holds current replica count
	- if metrics are unavailable (or no running pods are measured), it holds

### Deployment Configuration
You can tune autoscaling behavior with annotations on each Deployment:

```yaml
metadata:
	labels:
		autoscaler.yourorg.io/enabled: "true"
	annotations:
		autoscaler.yourorg.io/min-replicas: "2"
		autoscaler.yourorg.io/max-replicas: "20"
		autoscaler.yourorg.io/cpu-scale-up-threshold: "75"
		autoscaler.yourorg.io/cpu-scale-down-threshold: "25"
		autoscaler.yourorg.io/mem-scale-up-threshold: "80"
		autoscaler.yourorg.io/mem-scale-down-threshold: "30"
```

Important note:
- resource requests should be set on containers so CPU and memory percentages are meaningful

### Install in Local Cluster
The local cluster Makefile includes a target for installing the chart after the cluster and monitoring stack are up:

```bash
make install-autoscaler-operator
```
### Test

Run configuration command:
```bash
kubectl label deployment quote-app -n apps autoscaler.yourorg.io/enabled=true --overwrite

kubectl annotate deployment quote-app -n apps \
  autoscaler.yourorg.io/min-replicas=1 \
  autoscaler.yourorg.io/max-replicas=5 \
  autoscaler.yourorg.io/scale-up-step=1 \
  autoscaler.yourorg.io/scale-down-step=1 \
  autoscaler.yourorg.io/cpu-enabled=true \
  autoscaler.yourorg.io/cpu-scale-up-threshold=75 \
  autoscaler.yourorg.io/cpu-scale-down-threshold=25 \
  --overwrite
```

And then run the following command
```bash
kubectl run load-gen --image=busybox:1.28 --restart=Never -it --rm -n apps -- /bin/sh -c "while true; do wget -q -O- http://quote-app:8080/quote; done"
```
