# kubeprovisioner

A Go CLI for inspecting and diagnosing Kubernetes cluster state.
Built to simulate the kind of internal diagnostics tooling a platform team
maintains for customer deployed (BYOC) and SaaS-managed clusters.

Four subcommands:

| Command | What it does |
|---|---|
| `status` | Prints node and pod health across all namespaces |
| `diagnose` | Surfaces pods with high restart counts OOMKills or pending state |
| `report` | Emits a structured JSON report of cluster state (pipe to a file or monitoring system) |
| `check dns` | Resolves a hostname from inside the cluster  catches DNS misconfigurations before they become support tickets |

---

## Why I built this

When deploying ClusterGuard to a customer environment I kept running the same
sequence of `kubectl` commands to verify the cluster was healthy before and after
an install. This CLI wraps that sequence into four composable subcommands, outputs
structured JSON for easy parsing, and includes a DNS check that catches the most
common networking misconfiguration in BYOC installs (missing cluster DNS records
or a blocked port on the customer's VPC).

---

## Project structure

```
kubeprovisioner/
├── cmd/
│   └── kp/
│       └── main.go            # cobra root command + subcommand wiring
├── internal/
│   ├── kube/
│   │   ├── client.go          # client-go kubeconfig loader
│   │   ├── nodes.go           # node state queries
│   │   └── pods.go            # pod state queries
│   ├── diagnose/
│   │   ├── diagnose.go        # restart count + OOMKill + pending detection
│   │   └── diagnose_test.go
│   ├── report/
│   │   └── report.go          # JSON report builder
│   └── netcheck/
│       ├── dns.go             # DNS resolution + HTTP health check
│       └── dns_test.go
├── go.mod
└── README.md
```

---

## Installation

```bash
git clone https://github.com/RiyaJ6/kubeprovisioner
cd kubeprovisioner
go build -o kp ./cmd/kp
```

Or install directly:

```bash
go install github.com/RiyaJ6/kubeprovisioner/cmd/kp@latest
```

---

## Usage

All commands use your current kubeconfig context by default.
Pass `--kubeconfig` or `--context` to override.

### status

```bash
kp status

# output
NODES
  node/worker-1   Ready     2d3h
  node/worker-2   Ready     2d3h
  node/worker-3   NotReady  4m12s  ← flagged

PODS (non-Running)
  kube-system/coredns-abc123    Pending   0/1   2m
```

### diagnose

```bash
kp diagnose --restarts 5

# output
PODS WITH HIGH RESTART COUNTS (threshold: 5)
  default/clusterguard-7d9f6   restarts=8   last_reason=OOMKilled   container=clusterguard
  monitoring/prometheus-0       restarts=6   last_reason=Error       container=prometheus

PENDING PODS
  default/my-job-xyz   pending since 8m32s   reason: Unschedulable (insufficient memory)
```

### report

```bash
kp report --output cluster-state.json

# or pipe
kp report | jq '.pods[] | select(.restarts > 3)'
```

Report schema:

```json
{
  "generated_at": "2025-03-14T09:26:00Z",
  "context": "my-cluster",
  "nodes": [
    { "name": "worker-1", "ready": true, "age": "2d3h", "cpu_capacity": "4", "memory_capacity": "8Gi" }
  ],
  "pods": [
    {
      "namespace": "default",
      "name": "clusterguard-7d9f6",
      "phase": "Running",
      "restarts": 8,
      "last_termination_reason": "OOMKilled",
      "containers": ["clusterguard"]
    }
  ],
  "summary": {
    "total_nodes": 3,
    "not_ready_nodes": 1,
    "total_pods": 42,
    "pods_not_running": 2,
    "high_restart_pods": 1
  }
}
```

### check dns

```bash
kp check dns --host kafka.internal --port 9092

# output
✓  kafka.internal resolves to 10.0.1.45
✓  TCP connect to 10.0.1.45:9092 succeeded (12ms)

kp check dns --host kafka.internal --port 9092 --http --path /health

✓  kafka.internal resolves to 10.0.1.45
✓  TCP connect to 10.0.1.45:9092 succeeded (12ms)
✗  HTTP GET http://kafka.internal:9092/health returned 000 (connection refused)
```

---

## Running tests

```bash
go test ./... -v -race
```

The test suite uses table driven tests and does not require a live cluster.
Kubernetes API responses are stubbed using `k8s.io/client-go/kubernetes/fake`.

---

## Flags reference

```
Global flags:
  --kubeconfig string   path to kubeconfig (default: $KUBECONFIG or ~/.kube/config)
  --context string      kubeconfig context to use

kp diagnose:
  --restarts int        minimum restart count to flag (default 5)
  --namespace string    filter to a specific namespace (default: all)

kp report:
  --output string       write JSON to file instead of stdout

kp check dns:
  --host string         hostname to resolve (required)
  --port int            TCP port to check (default 80)
  --http                also send an HTTP GET request
  --path string         HTTP path (default /)
  --timeout duration    timeout for each check (default 5s)
```
<div align="center">

**All you can do and should do**
