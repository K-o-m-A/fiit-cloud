output "control_plane_floating_ip" {
  description = "Floating IP assigned to the control plane"
  value       = openstack_networking_floatingip_v2.control_plane_fip.address
}

output "control_plane_fixed_ip" {
  description = "Internal IP of the control plane"
  value       = openstack_networking_port_v2.control_plane_port.all_fixed_ips[0]
}

output "worker_fixed_ip" {
  description = "Internal IP of the worker node"
  value       = openstack_networking_port_v2.worker_port.all_fixed_ips[0]
}

output "ssh_control_plane" {
  description = "SSH command to access the control plane"
  value       = "ssh ubuntu@${openstack_networking_floatingip_v2.control_plane_fip.address}"
}