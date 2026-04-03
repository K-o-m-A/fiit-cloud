terraform {
  required_providers {
    openstack = {
      source  = "terraform-provider-openstack/openstack"
      version = "1.54.1"
    }
    null  = { source = "hashicorp/null" }
    local = { source = "hashicorp/local" }
  }
}

provider "openstack" {}

# ─────────────────────────────────────────────
# Network
# ─────────────────────────────────────────────

resource "openstack_networking_network_v2" "k8s_network" {
  name           = "k8s-network"
  admin_state_up = true
}

resource "openstack_networking_subnet_v2" "k8s_subnet" {
  name            = "k8s-subnet"
  network_id      = openstack_networking_network_v2.k8s_network.id
  cidr            = var.subnet_cidr
  ip_version      = 4
  dns_nameservers = ["8.8.8.8", "8.8.4.4"]
}

# ─────────────────────────────────────────────
# Router
# ─────────────────────────────────────────────

data "openstack_networking_network_v2" "external" {
  name = var.external_network_name
}

resource "openstack_networking_router_v2" "k8s_router" {
  name                = "k8s-router"
  admin_state_up      = true
  external_network_id = data.openstack_networking_network_v2.external.id
}

resource "openstack_networking_router_interface_v2" "k8s_router_iface" {
  router_id = openstack_networking_router_v2.k8s_router.id
  subnet_id = openstack_networking_subnet_v2.k8s_subnet.id
}

# ─────────────────────────────────────────────
# Security Group
# ─────────────────────────────────────────────

resource "openstack_networking_secgroup_v2" "k8s_sg" {
  name        = "k8s-security-group"
  description = "Security group for Kubernetes nodes"
}

resource "openstack_networking_secgroup_rule_v2" "internal_ingress" {
  direction         = "ingress"
  ethertype         = "IPv4"
  remote_group_id   = openstack_networking_secgroup_v2.k8s_sg.id
  security_group_id = openstack_networking_secgroup_v2.k8s_sg.id
}

resource "openstack_networking_secgroup_rule_v2" "ssh" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 22
  port_range_max    = 22
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.k8s_sg.id
}

resource "openstack_networking_secgroup_rule_v2" "icmp" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "icmp"
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.k8s_sg.id
}

resource "openstack_networking_secgroup_rule_v2" "k8s_api" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 6443
  port_range_max    = 6443
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.k8s_sg.id
}

resource "openstack_networking_secgroup_rule_v2" "etcd" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 2379
  port_range_max    = 2380
  remote_group_id   = openstack_networking_secgroup_v2.k8s_sg.id
  security_group_id = openstack_networking_secgroup_v2.k8s_sg.id
}

resource "openstack_networking_secgroup_rule_v2" "kubelet" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 10250
  port_range_max    = 10250
  remote_group_id   = openstack_networking_secgroup_v2.k8s_sg.id
  security_group_id = openstack_networking_secgroup_v2.k8s_sg.id
}

resource "openstack_networking_secgroup_rule_v2" "scheduler" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 10259
  port_range_max    = 10259
  remote_group_id   = openstack_networking_secgroup_v2.k8s_sg.id
  security_group_id = openstack_networking_secgroup_v2.k8s_sg.id
}

resource "openstack_networking_secgroup_rule_v2" "controller_manager" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 10257
  port_range_max    = 10257
  remote_group_id   = openstack_networking_secgroup_v2.k8s_sg.id
  security_group_id = openstack_networking_secgroup_v2.k8s_sg.id
}

resource "openstack_networking_secgroup_rule_v2" "nodeport" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "tcp"
  port_range_min    = 30000
  port_range_max    = 32767
  remote_ip_prefix  = "0.0.0.0/0"
  security_group_id = openstack_networking_secgroup_v2.k8s_sg.id
}

resource "openstack_networking_secgroup_rule_v2" "flannel_vxlan" {
  direction         = "ingress"
  ethertype         = "IPv4"
  protocol          = "udp"
  port_range_min    = 8472
  port_range_max    = 8472
  remote_group_id   = openstack_networking_secgroup_v2.k8s_sg.id
  security_group_id = openstack_networking_secgroup_v2.k8s_sg.id
}

# ─────────────────────────────────────────────
# Ports
# ─────────────────────────────────────────────

resource "openstack_networking_port_v2" "control_plane_port" {
  name               = "control-plane-port"
  network_id         = openstack_networking_network_v2.k8s_network.id
  admin_state_up     = true
  security_group_ids = [openstack_networking_secgroup_v2.k8s_sg.id]

  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.k8s_subnet.id
  }

  allowed_address_pairs {
    ip_address = "10.244.0.0/16"
  }
}

resource "openstack_networking_port_v2" "worker_port" {
  name               = "worker-port"
  network_id         = openstack_networking_network_v2.k8s_network.id
  admin_state_up     = true
  security_group_ids = [openstack_networking_secgroup_v2.k8s_sg.id]

  fixed_ip {
    subnet_id = openstack_networking_subnet_v2.k8s_subnet.id
  }

  allowed_address_pairs {
    ip_address = "10.244.0.0/16"
  }
}

# ─────────────────────────────────────────────
# Compute Instances
# ─────────────────────────────────────────────

data "openstack_images_image_v2" "ubuntu" {
  name        = var.image_name
  most_recent = true
}

resource "openstack_compute_instance_v2" "control_plane" {
  name        = "k8s-control-plane"
  flavor_name = var.control_plane_flavor
  key_pair    = var.keypair_name
  image_id    = data.openstack_images_image_v2.ubuntu.id

  network {
    port = openstack_networking_port_v2.control_plane_port.id
  }

  metadata = {
    role = "control-plane"
  }
}

resource "openstack_compute_instance_v2" "worker" {
  name        = "k8s-worker-01"
  flavor_name = var.worker_flavor
  key_pair    = var.keypair_name
  image_id    = data.openstack_images_image_v2.ubuntu.id

  network {
    port = openstack_networking_port_v2.worker_port.id
  }

  metadata = {
    role = "worker"
  }
}

# ─────────────────────────────────────────────
# Floating IP — Control Plane only
# ─────────────────────────────────────────────

resource "openstack_networking_floatingip_v2" "control_plane_fip" {
  pool = var.external_network_name
}

resource "openstack_networking_floatingip_associate_v2" "control_plane_fip_assoc" {
  floating_ip = openstack_networking_floatingip_v2.control_plane_fip.address
  port_id     = openstack_networking_port_v2.control_plane_port.id

  depends_on = [openstack_networking_router_interface_v2.k8s_router_iface]
}

# ─────────────────────────────────────────────
# Locals — resolved IPs for provisioners
# ─────────────────────────────────────────────

locals {
  control_plane_fip         = openstack_networking_floatingip_v2.control_plane_fip.address
  control_plane_internal_ip = openstack_networking_port_v2.control_plane_port.all_fixed_ips[0]
  worker_internal_ip        = openstack_networking_port_v2.worker_port.all_fixed_ips[0]
}

# ─────────────────────────────────────────────
# Control Plane Bootstrap
# ─────────────────────────────────────────────

resource "null_resource" "control_plane_init" {
  depends_on = [openstack_networking_floatingip_associate_v2.control_plane_fip_assoc]

  connection {
    type        = "ssh"
    host        = local.control_plane_fip
    user        = var.ssh_user
    private_key = file(pathexpand(var.ssh_private_key_path))
  }

  provisioner "file" {
    content = templatefile("${path.module}/templates/control-plane-init.sh.tpl", {
      internal_ip  = local.control_plane_internal_ip
      pod_cidr     = var.pod_cidr
      service_cidr = var.service_cidr
    })
    destination = "/tmp/control-plane-init.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/control-plane-init.sh",
      "sudo bash -o pipefail -c '/tmp/control-plane-init.sh 2>&1 | tee /tmp/k8s-init.log'",
      "sudo kubeadm token create --print-join-command > /tmp/join-command.txt"
    ]
  }
}

# ─────────────────────────────────────────────
# Worker Join
# ─────────────────────────────────────────────

resource "null_resource" "worker_join" {
  depends_on = [null_resource.control_plane_init]

  connection {
    type                = "ssh"
    user                = var.ssh_user
    host                = local.worker_internal_ip
    private_key         = file(pathexpand(var.ssh_private_key_path))
    bastion_host        = local.control_plane_fip
    bastion_user        = var.ssh_user
    bastion_private_key = file(pathexpand(var.ssh_private_key_path))
  }

  provisioner "file" {
    content = templatefile("${path.module}/templates/worker-join.sh.tpl", {
      internal_ip = local.worker_internal_ip
    })
    destination = "/tmp/worker-join.sh"
  }

  provisioner "remote-exec" {
    inline = [
      "chmod +x /tmp/worker-join.sh",
      "sudo bash -o pipefail -c '/tmp/worker-join.sh 2>&1 | tee /tmp/k8s-worker-setup.log'"
    ]
  }

  provisioner "local-exec" {
    command = <<-EOT
      JOIN_CMD=$(ssh -i ${var.ssh_private_key_path} \
                     -o StrictHostKeyChecking=no \
                     ${var.ssh_user}@${local.control_plane_fip} \
                     "cat /tmp/join-command.txt")

      ssh -i ${var.ssh_private_key_path} \
          -o StrictHostKeyChecking=no \
          -o ProxyCommand=\"ssh -i ${var.ssh_private_key_path} -o StrictHostKeyChecking=no -W %h:%p ${var.ssh_user}@${local.control_plane_fip}\" \
          ${var.ssh_user}@${local.worker_internal_ip} \
          "sudo $JOIN_CMD"
    EOT
  }
}

# ─────────────────────────────────────────────
# Fetch kubeconfig
# ─────────────────────────────────────────────

resource "null_resource" "fetch_kubeconfig" {
  depends_on = [null_resource.worker_join]

  provisioner "local-exec" {
    command = <<-EOT
      ssh -i ${var.ssh_private_key_path} \
          -o StrictHostKeyChecking=no \
          ${var.ssh_user}@${local.control_plane_fip} \
          "sudo cat /etc/kubernetes/admin.conf" > ~/.kube/openstack-k8s.conf
      chmod 600 ~/.kube/openstack-k8s.conf
      sed -i 's|https://10\.[0-9]*\.[0-9]*\.[0-9]*|https://${local.control_plane_fip}|g' \
          ~/.kube/openstack-k8s.conf
    EOT
  }
}