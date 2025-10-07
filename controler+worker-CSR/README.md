# Kubernetes Cluster Setup with CSR Authentication

Complete guide for setting up a Kubernetes cluster with worker nodes registered via Certificate Signing Request (CSR) - the most secure method.

## 📋 Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Architecture](#architecture)
- [Installation Steps](#installation-steps)
  - [Step 1: Control Plane Setup](#step-1-control-plane-setup)
  - [Step 2: Worker Node Preparation](#step-2-worker-node-preparation)
  - [Step 3: CSR Registration](#step-3-csr-registration)
  - [Step 4: CSR Approval](#step-4-csr-approval)
  - [Step 5: Final Configuration](#step-5-final-configuration)
- [Verification](#verification)
- [Troubleshooting](#troubleshooting)
- [Useful Commands](#useful-commands)

---

## 🎯 Overview

This guide demonstrates setting up a Kubernetes 1.34 cluster with:
- **Control Plane**: Manages the cluster
- **Worker Node**: Registered via CSR for maximum security
- **CNI**: Flannel for pod networking
- **Container Runtime**: containerd

### Security Levels

1. **Basic**: Standard kubeadm join with token
2. **Advanced**: kubeconfig-based registration
3. **Maximum (This Guide)**: CSR-based certificate authentication ✅

---

## ⚙️ Prerequisites

### Hardware Requirements

**Control Plane:**
- CPU: 2+ cores
- RAM: 2 GB minimum (1 GB can work with warnings)
- Disk: 20 GB

**Worker Node:**
- CPU: 1+ core
- RAM: 1 GB minimum
- Disk: 20 GB

### Software Requirements

- Ubuntu 24.04 LTS (Noble)
- Root or sudo access
- Network connectivity between nodes
- Open ports: 6443 (API server), 10250 (kubelet)

### Network Information Needed

- Control Plane IP address (e.g., `10.0.1.93`)
- Worker Node IP address (e.g., `10.0.9.223`)

---

## 🏗️ Architecture

```
┌─────────────────────────────┐
│   Control Plane Node        │
│   (ip-10-0-1-93)            │
│                             │
│   - kube-apiserver          │
│   - kube-controller-manager │
│   - kube-scheduler          │
│   - etcd                    │
│   - Flannel CNI             │
└──────────────┬──────────────┘
               │ CSR/Certificate
               │ Authentication
┌──────────────▼──────────────┐
│   Worker Node               │
│   (ip-10-0-9-223)           │
│                             │
│   - kubelet                 │
│   - kube-proxy              │
│   - containerd              │
│   - Flannel agent           │
└─────────────────────────────┘
```

---

## 🚀 Installation Steps

### Step 1: Control Plane Setup

#### 1.1 Download and Prepare Script

On the control plane node:

```bash
# Download the setup script
wget https://your-script-location/setup-k8s.sh

# Make it executable
chmod +x setup-k8s.sh
```

#### 1.2 Run Full Control Plane Setup

```bash
# Run the script
sudo ./setup-k8s.sh

# Select option 6: Full Control Plane setup (1+2+3)
```

This will:
- Install all dependencies (containerd, kernel modules, etc.)
- Install Kubernetes tools (kubeadm, kubelet, kubectl v1.34)
- Initialize the control plane
- Install Flannel CNI
- Generate join token

**Expected output:**
```
[INFO] Control Plane initialized successfully!
[INFO] Join command saved to /root/join-command.sh
```

#### 1.3 Save Important Information

```bash
# Get the CA certificate (needed for worker)
sudo cat /etc/kubernetes/pki/ca.crt

# Get the API server address
kubectl cluster-info

# Note: API Server is at https://10.0.1.93:6443
```

**Save these values:**
- ✅ CA certificate content
- ✅ API server URL (https://YOUR_CONTROL_PLANE_IP:6443)

---

### Step 2: Worker Node Preparation

#### 2.1 Download Setup Script

On the worker node:

```bash
# Download the same setup script
wget https://your-script-location/setup-k8s.sh
chmod +x setup-k8s.sh
```

#### 2.2 Install Dependencies and Tools

```bash
sudo ./setup-k8s.sh

# First, select option 1: Install dependencies
# Wait for completion...

# Then, select option 2: Install Kubernetes tools
```

#### 2.3 Setup CA Certificate

```bash
# Create PKI directory
sudo mkdir -p /etc/kubernetes/pki

# Create CA certificate file
sudo nano /etc/kubernetes/pki/ca.crt

# Paste the CA certificate content from control plane
# Save: Ctrl+O, Enter, Ctrl+X
```

---

### Step 3: CSR Registration

#### 3.1 Generate CSR on Worker Node

```bash
# Get node information
NODE_NAME=$(hostname)
NODE_IP=$(hostname -I | awk '{print $1}')

echo "Node name: $NODE_NAME"
echo "Node IP: $NODE_IP"

# Generate private key
sudo openssl genrsa -out /etc/kubernetes/pki/worker-${NODE_NAME}.key 2048

# Create CSR configuration
cat <<EOF | sudo tee /tmp/csr.conf
[req]
req_extensions = v3_req
distinguished_name = req_distinguished_name
[req_distinguished_name]
[ v3_req ]
basicConstraints = CA:FALSE
keyUsage = nonRepudiation, digitalSignature, keyEncipherment
subjectAltName = @alt_names
[alt_names]
DNS.1 = ${NODE_NAME}
IP.1 = ${NODE_IP}
EOF

# Generate Certificate Signing Request
sudo openssl req -new \
    -key /etc/kubernetes/pki/worker-${NODE_NAME}.key \
    -out /tmp/worker-${NODE_NAME}.csr \
    -subj "/CN=system:node:${NODE_NAME}/O=system:nodes" \
    -config /tmp/csr.conf

# Display base64 encoded CSR
echo "=== Copy this base64 CSR ==="
sudo cat /tmp/worker-${NODE_NAME}.csr | base64 | tr -d '\n'
echo ""
echo "==========================="
```

**Copy the base64 CSR output** - you'll need it on the control plane.

---

### Step 4: CSR Approval

#### 4.1 Create CSR Object on Control Plane

On the control plane node:

```bash
# Create CSR YAML file (replace <NODE_NAME> and <BASE64_CSR>)
cat <<EOF > node-csr.yaml
apiVersion: certificates.k8s.io/v1
kind: CertificateSigningRequest
metadata:
  name: node-csr-<NODE_NAME>
spec:
  request: <BASE64_CSR>
  signerName: kubernetes.io/kube-apiserver-client
  usages:
  - client auth
EOF

# Apply the CSR
kubectl apply -f node-csr.yaml
```

#### 4.2 Approve CSR

```bash
# View pending CSRs
kubectl get csr

# Approve the CSR (replace <NODE_NAME>)
kubectl certificate approve node-csr-<NODE_NAME>

# Verify approval
kubectl get csr
# Should show: Approved,Issued

# Extract signed certificate
kubectl get csr node-csr-<NODE_NAME> -o jsonpath='{.status.certificate}' | base64 -d > worker-cert.crt

# Display certificate
cat worker-cert.crt
```

**Copy the certificate content** - needed for worker node.

---

### Step 5: Final Configuration

#### 5.1 Install Certificate on Worker Node

On the worker node:

```bash
# Create certificate file
sudo nano /etc/kubernetes/pki/worker-$(hostname).crt

# Paste the certificate content from control plane
# Save and exit
```

#### 5.2 Configure Kubelet

```bash
# Create kubelet configuration directory
sudo mkdir -p /var/lib/kubelet

# Create kubelet config
sudo nano /var/lib/kubelet/config.yaml
```

Paste this content:

```yaml
apiVersion: kubelet.config.k8s.io/v1beta1
kind: KubeletConfiguration
authentication:
  anonymous:
    enabled: false
  webhook:
    enabled: true
  x509:
    clientCAFile: /etc/kubernetes/pki/ca.crt
authorization:
  mode: Webhook
clusterDomain: cluster.local
clusterDNS:
  - 10.96.0.10
cgroupDriver: systemd
```

#### 5.3 Create Bootstrap Kubeconfig

```bash
# Create bootstrap kubeconfig (replace <CONTROL_PLANE_IP> with your IP)
sudo nano /etc/kubernetes/bootstrap-kubelet.conf
```

Paste (replace `<CONTROL_PLANE_IP>` and `<NODE_NAME>`):

```yaml
apiVersion: v1
kind: Config
clusters:
- cluster:
    certificate-authority: /etc/kubernetes/pki/ca.crt
    server: https://<CONTROL_PLANE_IP>:6443
  name: kubernetes
contexts:
- context:
    cluster: kubernetes
    user: system:node:<NODE_NAME>
  name: system:node:<NODE_NAME>@kubernetes
current-context: system:node:<NODE_NAME>@kubernetes
users:
- name: system:node:<NODE_NAME>
  user:
    client-certificate: /etc/kubernetes/pki/worker-<NODE_NAME>.crt
    client-key: /etc/kubernetes/pki/worker-<NODE_NAME>.key
```

#### 5.4 Configure Kubelet Service

```bash
# Create systemd override directory
sudo mkdir -p /etc/systemd/system/kubelet.service.d

# Create kubelet service configuration
sudo nano /etc/systemd/system/kubelet.service.d/10-kubeadm.conf
```

Paste this content:

```ini
[Service]
Environment="KUBELET_KUBECONFIG_ARGS=--bootstrap-kubeconfig=/etc/kubernetes/bootstrap-kubelet.conf --kubeconfig=/etc/kubernetes/kubelet.conf"
Environment="KUBELET_CONFIG_ARGS=--config=/var/lib/kubelet/config.yaml"
Environment="KUBELET_EXTRA_ARGS="
ExecStart=
ExecStart=/usr/bin/kubelet $KUBELET_KUBECONFIG_ARGS $KUBELET_CONFIG_ARGS $KUBELET_EXTRA_ARGS
```

#### 5.5 Start Kubelet

```bash
# Reload systemd
sudo systemctl daemon-reload

# Restart kubelet
sudo systemctl restart kubelet

# Check status
sudo systemctl status kubelet

# Should show: active (running)
```

#### 5.6 Setup kubectl Access (Optional)

To use kubectl on worker node:

```bash
# Create kube config directory
mkdir -p $HOME/.kube

# Copy admin config from control plane
# On control plane, run:
sudo cat /etc/kubernetes/admin.conf

# On worker node, create config:
nano $HOME/.kube/config
# Paste the content and save

# Set permissions
chmod 600 $HOME/.kube/config
```

---

## ✅ Verification

### On Control Plane

```bash
# Check cluster status
kubectl cluster-info

# List all nodes
kubectl get nodes
# Expected output:
# NAME              STATUS   ROLES           AGE   VERSION
# ip-10-0-1-93      Ready    control-plane   30m   v1.34.1
# ip-10-0-9-223     Ready    <none>          5m    v1.34.1

# Detailed node information
kubectl get nodes -o wide

# Check all pods
kubectl get pods -A

# Check CSR status
kubectl get csr
# Should show: Approved,Issued for both nodes
```

### Test Deployment

```bash
# Create test deployment
kubectl create deployment nginx --image=nginx --replicas=2

# Check pods
kubectl get pods -o wide

# Expose service
kubectl expose deployment nginx --port=80 --type=NodePort

# Get service details
kubectl get svc nginx

# Test access
curl http://<NODE_IP>:<NODE_PORT>

# Cleanup
kubectl delete deployment nginx
kubectl delete service nginx
```

---

## 🔧 Troubleshooting

### Worker Node Not Appearing

```bash
# On worker node, check kubelet logs
sudo journalctl -u kubelet -f

# Check kubelet status
sudo systemctl status kubelet

# Verify certificates exist
ls -la /etc/kubernetes/pki/

# Should have:
# - ca.crt
# - worker-<hostname>.key
# - worker-<hostname>.crt
```

### Connection Refused Errors

```bash
# Check network connectivity
ping <CONTROL_PLANE_IP>

# Test API server access
curl -k https://<CONTROL_PLANE_IP>:6443

# Check firewall
sudo ufw status

# Allow API server port
sudo ufw allow 6443/tcp
```

### CSR Not Auto-Approved

```bash
# Manual approval on control plane
kubectl get csr
kubectl certificate approve <CSR_NAME>

# Or approve all pending
kubectl get csr -o name | xargs kubectl certificate approve
```

### Node Status: NotReady

```bash
# Check CNI pods
kubectl get pods -n kube-system | grep flannel

# Restart flannel if needed
kubectl delete pod -n kube-system -l app=flannel

# Check node conditions
kubectl describe node <NODE_NAME>
```

### Low Memory Warnings

The script ignores memory checks with `--ignore-preflight-errors=Mem,NumCPU`. For production:

- **Minimum**: 2 GB RAM per node
- **Recommended**: 4 GB+ RAM

To add more memory on AWS:
1. Stop instance
2. Change instance type
3. Start instance

---

## 📚 Useful Commands

### Cluster Management

```bash
# Cluster information
kubectl cluster-info
kubectl cluster-info dump

# Node management
kubectl get nodes
kubectl describe node <NODE_NAME>
kubectl top nodes  # requires metrics-server

# Label nodes
kubectl label node <NODE_NAME> node-role.kubernetes.io/worker=worker

# Drain node (for maintenance)
kubectl drain <NODE_NAME> --ignore-daemonsets
kubectl uncordon <NODE_NAME>
```

### Pod Management

```bash
# List all pods
kubectl get pods -A
kubectl get pods -A -o wide

# Pod logs
kubectl logs -n <NAMESPACE> <POD_NAME>
kubectl logs -n <NAMESPACE> <POD_NAME> -f  # follow

# Execute command in pod
kubectl exec -it <POD_NAME> -- /bin/bash
```

### Certificate Management

```bash
# Check certificate expiration
kubeadm certs check-expiration

# Renew certificates
sudo kubeadm certs renew all

# View CSR details
kubectl get csr <CSR_NAME> -o yaml
```

### Debugging

```bash
# Cluster events
kubectl get events -A --sort-by='.lastTimestamp'

# Component status
kubectl get componentstatuses

# Node logs
sudo journalctl -u kubelet -f
sudo journalctl -u containerd -f

# API server logs
kubectl logs -n kube-system kube-apiserver-<CONTROL_PLANE>
```

---

## 🎓 Additional Features

### Install Metrics Server

```bash
kubectl apply -f https://github.com/kubernetes-sigs/metrics-server/releases/latest/download/components.yaml

# For low-resource environments, patch it:
kubectl patch deployment metrics-server -n kube-system --type='json' \
  -p='[{"op": "add", "path": "/spec/template/spec/containers/0/args/-", "value": "--kubelet-insecure-tls"}]'
```

### Install Dashboard (Optional)

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes/dashboard/v2.7.0/aio/deploy/recommended.yaml

# Create admin user
kubectl create serviceaccount dashboard-admin -n kubernetes-dashboard
kubectl create clusterrolebinding dashboard-admin \
  --clusterrole=cluster-admin \
  --serviceaccount=kubernetes-dashboard:dashboard-admin

# Get token
kubectl -n kubernetes-dashboard create token dashboard-admin

# Access via proxy
kubectl proxy
# Then visit: http://localhost:8001/api/v1/namespaces/kubernetes-dashboard/services/https:kubernetes-dashboard:/proxy/
```

### Add More Worker Nodes

Repeat steps 2-5 for each additional worker node.

---

## 📝 Summary

You have successfully set up a Kubernetes cluster with:

✅ Kubernetes 1.34  
✅ Control Plane with kubeadm  
✅ Worker Node registered via CSR (maximum security)  
✅ Flannel CNI for networking  
✅ containerd runtime  

### Key Security Features

- **Certificate-based authentication**: More secure than token-based
- **Individual node certificates**: Each node has unique credentials
- **Manual CSR approval**: Control plane administrator approves each node
- **No shared secrets**: No cluster-wide join tokens

---

## 📖 References

- [Kubernetes Official Documentation](https://kubernetes.io/docs/)
- [kubeadm Documentation](https://kubernetes.io/docs/setup/production-environment/tools/kubeadm/)
- [Certificate Signing Requests](https://kubernetes.io/docs/reference/access-authn-authz/certificate-signing-requests/)
- [Flannel CNI](https://github.com/flannel-io/flannel)
- [containerd](https://containerd.io/)

---

## 👥 Support

For issues or questions:
1. Check the [Troubleshooting](#troubleshooting) section
2. Review Kubernetes logs: `sudo journalctl -u kubelet -f`
3. Consult official Kubernetes documentation
4. Check GitHub issues for known problems

---

**Version**: 1.0  
**Kubernetes Version**: 1.34.1  
**Last Updated**: October 2025  
**License**: MIT
