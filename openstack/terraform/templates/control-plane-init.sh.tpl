#!/bin/bash
set -euxo pipefail

# Container runtime
apt-get update -qq
apt-get install -y containerd apt-transport-https curl ca-certificates gnupg

install -d -m 0755 /etc/apt/keyrings

mkdir -p /etc/containerd
containerd config default > /etc/containerd/config.toml
sed -i 's/SystemdCgroup = false/SystemdCgroup = true/' /etc/containerd/config.toml
systemctl restart containerd
systemctl enable containerd

# kubeadm requires swap to be disabled
swapoff -a
sed -ri '/\sswap\s/s/^#?/#/' /etc/fstab

# Add Kubernetes apt repo

curl -fsSL "https://pkgs.k8s.io/core:/stable:/v1.32/deb/Release.key" \
  | gpg --dearmor --batch --yes --output /etc/apt/keyrings/kubernetes-apt-keyring.gpg

echo "deb [signed-by=/etc/apt/keyrings/kubernetes-apt-keyring.gpg] \
https://pkgs.k8s.io/core:/stable:/v1.32/deb/ /" \
  | tee /etc/apt/sources.list.d/kubernetes.list

apt-get update
apt-get install -y kubelet kubeadm kubectl
apt-mark hold kubelet kubeadm kubectl
systemctl enable kubelet
command -v kubeadm >/dev/null

# Kernel settings
modprobe overlay br_netfilter
cat > /etc/sysctl.d/k8s.conf <<EOF
net.bridge.bridge-nf-call-iptables  = 1
net.bridge.bridge-nf-call-ip6tables = 1
net.ipv4.ip_forward                 = 1
EOF
sysctl --system

if [ ! -f /etc/kubernetes/admin.conf ]; then
  kubeadm init \
    --apiserver-advertise-address=${internal_ip} \
    --apiserver-cert-extra-sans=${internal_ip} \
    --node-name=$(hostname -s) \
    --pod-network-cidr=${pod_cidr} \
    --service-cidr=${service_cidr}
fi

# kubeconfig for ubuntu user
mkdir -p /home/ubuntu/.kube
cp /etc/kubernetes/admin.conf /home/ubuntu/.kube/config
chown ubuntu:ubuntu /home/ubuntu/.kube/config

# Flannel CNI
kubectl --kubeconfig=/etc/kubernetes/admin.conf \
  apply -f https://github.com/flannel-io/flannel/releases/latest/download/kube-flannel.yml