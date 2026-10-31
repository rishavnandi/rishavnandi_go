---
title: "A Three-Node K3s Cluster on OCI's Free Tier"
date: "2026-10-31"
description: "Three Always Free OCI instances, provisioned with Terraform and configured with Ansible, mixed ARM and AMD64."
category: 'Tutorial'
writing: false
published: true
tags: ["kubernetes", "k3s", "oci", "terraform", "ansible"]
---

Three Always Free OCI instances, one K3s server that also runs workloads, and two small workers. Terraform builds the network and the hosts. Ansible installs K3s and verifies it. The code is in [rishavnandi/oci_k3s](https://github.com/rishavnandi/oci_k3s).

## The three nodes

| Node | Shape | Arch | Role |
|---|---|---|---|
| `loki` | `VM.Standard.A1.Flex`, 4 OCPU, 24 GB, 100 GB boot | arm64 | Server and worker |
| `heimdall` | `VM.Standard.E2.1.Micro`, 1 GB, 50 GB boot | amd64 | Worker |
| `thor` | `VM.Standard.E2.1.Micro`, 1 GB, 50 GB boot | amd64 | Worker |

All three run Ubuntu 24.04 in `uk-london-1` (UK South), in a single availability domain.

## Why the free tier allows exactly this

Always Free gives 200 GB of block volume in total. The three boot volumes are 100, 50 and 50 GB, so they use the whole allowance. There is no room for a fourth disk or for a volume left behind by an earlier attempt. Clear old boot volumes before you apply, or the plan fails on quota. Terraform sets `preserve_boot_volume = false`, so a destroy releases them.

The A1 shape takes the full Ampere allowance of 4 OCPU and 24 GB. The two micro instances take the two free micro slots.

## Provisioning with Terraform

Generate the shared key first. Terraform installs the public half on every node.

```bash
mkdir -p .secrets
ssh-keygen -t ed25519 -f .secrets/oci_k3s -N '' -C oci-k3s
```

Fill `terraform/terraform.tfvars` with the tenancy and compartment OCIDs, `region = "uk-london-1"`, the public key, and a Tailscale auth key. The provider uses `auth = "SecurityToken"`, so authenticate the CLI with `oci session authenticate` before the first plan.

```bash
terraform -chdir=terraform init
terraform -chdir=terraform plan -var-file=terraform.tfvars -out=cluster.tfplan
terraform -chdir=terraform apply cluster.tfplan
./scripts/write_inventory.sh
```

The configuration creates a VCN at 10.0.0.0/16 with one public subnet at 10.0.0.0/24. The security list allows SSH from anywhere and all traffic between nodes inside the VCN. Nothing else is open at the OCI layer.

`write_inventory.sh` writes the `ansible_inventory` output to `ansible/inventory.ini`, with a public IP and a private IP for each host.

## Configuring with Ansible

```bash
cd ansible
ansible-playbook site.yml
```

The first play prepares all three hosts and sets UFW rules for the API (6443), kubelet (10250) and VXLAN (8472) from the cluster subnet. The second installs the server on `loki` with `--disable=traefik` and `--disable=servicelb`, waits for the API on 6443, and reads the worker token. The third joins `heimdall` and then `thor`, one at a time, against `https://<loki-private-ip>:6443`.

The last play verifies the result. It waits up to 300 seconds for `loki`, `heimdall` and `thor` to reach `Ready`, then prints `k3s kubectl get nodes -o wide`. A node that never joins fails the playbook.

For local `kubectl`, fetch the admin kubeconfig from the server:

```bash
mkdir -p ~/.kube
ssh -i .secrets/oci_k3s ubuntu@<loki-public-ip> 'sudo cat /etc/rancher/k3s/k3s.yaml' > ~/.kube/oci-k3s.yaml
chmod 600 ~/.kube/oci-k3s.yaml
```

That file points at Loki's private IP. Change the server URL to an address your machine can reach before you use it.

## Two architectures in one cluster

`loki` is arm64. The two workers are amd64. The scheduler does not care, but container images do. An amd64-only image fails to start on `loki` with an exec format error.

There are two ways out. Publish multi-arch images with a manifest list, which is the cleaner default. Or pin a workload to one architecture:

```yaml
nodeSelector:
  kubernetes.io/arch: amd64
```

Keep substantial workloads on `loki`. The two micro workers have 1 GB of memory each, which is enough for small agents and not much more.

## What the install leaves out

K3s runs without Traefik and without ServiceLB, so there is no bundled ingress controller and no service load balancer. A `LoadBalancer` Service stays pending until you install something that handles it.

Tailscale is outside the cluster on purpose. The Terraform cloud-init joins each host to the tailnet, which gives you a private path to SSH and the API, and UFW allows traffic on `tailscale0`. The Tailscale Kubernetes Operator is a separate layer with its own OAuth credentials, and this project does not install it.

## Removing it

`ansible-playbook teardown.yml` runs the K3s uninstallers and leaves the hosts in place. `terraform -chdir=terraform destroy -var-file=terraform.tfvars` removes the instances and the network.
