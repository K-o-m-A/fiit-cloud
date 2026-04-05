# FIIT-CLOUD-PROJECT
### Authors: Peter Farkaš, Dárius=Dušan Horváth, Adrián Komanek, Frederik Duvač

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
