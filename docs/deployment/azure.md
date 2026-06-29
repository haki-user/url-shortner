# Azure Container Apps Deployment

## Topology

The cost-optimized student deployment uses managed, scale-to-zero application
compute:

```text
GitHub Actions
    |
    | OIDC + immutable image
    v
Azure Container Registry
    |
    | user-assigned managed identity
    v
Azure Container Apps Consumption
|-- tinyurl-migrate  manual one-shot job
`-- tinyurl-api      HTTPS ingress, 0-3 replicas
          |
          | custom domain + managed TLS
          v
    https://tinyurl.haki-user.in
          | \
          |  `-> private utility VM Redis at 10.20.4.4:6379
          |
          `-> private VNet + private DNS
                    |
                    v
Azure PostgreSQL Flexible Server B1ms
```

PostgreSQL remains authoritative. Redirect reads use a shared, private Redis
instance on the utility VM through the implemented versioned cache-aside
resolver. Redis is disposable: it has no persistence, uses a 128 MB memory cap
with `allkeys-lru` eviction, and can be rebuilt entirely from PostgreSQL.

## Cost Model

Container Apps Consumption can scale the API to zero replicas. Redis runs on a
single private `Standard_B2ats_v2` utility VM because Azure for Students lists
750 free hours/month for this VM size, which covers one always-on VM in a
normal month. The VM has no public IP, no Bastion, no backup, no extra data
disk, and boot diagnostics disabled.

The utility VM still creates one unavoidable managed OS disk:

- `tinyurl-utility-vm-osdisk`, `Standard_LRS`, 30 GB.

Do not add public IPs, Bastion, VPN Gateway, VM backups, snapshots, extra disks,
or larger VM SKUs without first checking Cost Analysis and the student free
service limits.

PostgreSQL, ACR, Key Vault, network usage, and usage beyond Container Apps
grants can still consume student credit. Check Cost Analysis and keep a budget
alert enabled.

## Security Boundaries

- Container Apps terminates HTTPS and exposes only the application ingress.
- `tinyurl.haki-user.in` is bound with an Azure-managed certificate.
- PostgreSQL remains in its delegated private subnet.
- Container Apps reaches PostgreSQL through a dedicated delegated `/27`
  subnet and the existing private DNS zone.
- Container Apps reaches Redis through the private VNet only. The utility VM
  NSG allows Redis from `VirtualNetwork` and exposes no public inbound path.
- The database URL remains in Azure Key Vault.
- A user-assigned managed identity receives only `AcrPull` and
  `Key Vault Secrets User`.
- GitHub authenticates to Azure through federated OIDC, not a client secret.
- Application authentication still uses the development `X-Owner-ID` model
  and must be replaced before treating the management API as public.

## Infrastructure

[`infra/azure/main.bicep`](../../infra/azure/main.bicep) defines the durable
foundation: PostgreSQL, ACR, Key Vault, private DNS, and the VNet.

[`infra/azure/container-apps.bicep`](../../infra/azure/container-apps.bicep)
adds the application resources to that foundation:

- delegated `container-apps` subnet at `10.20.3.0/27`;
- Consumption workload-profile environment;
- `tinyurl-api` container app with managed HTTPS and HTTP scaling from zero;
- `tinyurl-migrate` manually triggered migration job;
- user-assigned identity and narrow ACR/Key Vault role assignments.

[`infra/azure/utility-vm.bicep`](../../infra/azure/utility-vm.bicep) defines
the private utility VM:

- `Standard_B2ats_v2` Linux VM;
- static private IP `10.20.4.4`;
- no public IP;
- `utility` subnet at `10.20.4.0/24`;
- NSG allowing Redis only from the virtual network.

Provision against the immutable image already in ACR:

```powershell
$env:Path = "C:\Program Files\Microsoft SDKs\Azure\CLI2\wbin;$env:Path"

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\infra\azure\provision-container-apps.ps1 `
  -ImageTag <git-sha>
```

For a new environment, run
[`provision-foundation.ps1`](../../infra/azure/provision-foundation.ps1) once,
then run `provision-container-apps.ps1`. The foundation provisioner refuses to
run when PostgreSQL already exists so it cannot unexpectedly rotate the live
database password.

## Runtime Flow

Each production deployment:

```text
CI succeeds on main
-> GitHub obtains a short-lived Azure OIDC token
-> build and push tinyurl-linkd:<git-sha>
-> update and run tinyurl-migrate
-> wait for migration success
-> update tinyurl-api to the same image
-> Container Apps creates an immutable revision
-> verify /readyz through managed HTTPS ingress
```

The API uses `minReplicas: 0` and `maxReplicas: 3`. Zero minimizes idle cost
but introduces cold-start latency after an idle period. Redis uses exactly one
replica because scaling a stateful in-memory server to multiple independent
replicas would not create a coherent shared cache.

## Explicit Deployment Decisions

| Area | Student deployment | Production-scale target |
|---|---|---|
| API minimum replicas | Zero; accepts cold starts | One or more warm replicas |
| API maximum replicas | Three; cost guardrail | Load-tested limit with a database connection budget |
| Redis | Disposable Redis on private free-service-eligible VM | Azure Managed Redis with replication and Private Link |
| PostgreSQL | Single B1ms, seven-day backups | Zone-redundant HA, tested restores and read replicas |
| Logging | Application stdout only | Central logs, metrics, traces, alerts and retention |
| Region | One | Multi-region redirect reads and regional failover |
| Authentication | Temporary `X-Owner-ID` | Verified identity at the gateway or application boundary |
| Rate limiting | Not implemented | Per-principal creation and management limits |
| Domain | `tinyurl.haki-user.in` with managed TLS | Gateway-managed domain policy and certificate automation |

These are deployment tradeoffs, not missing parts of the target system design.
They must be revisited using measured traffic, latency, failure, and budget
data.

## CI/CD

Development and delivery flow:

```text
dev -> pull request to main -> CI -> review -> merge
                                      |
                                      v
                         successful main CI
                                      |
                                      v
                           production deployment
```

CI runs race-enabled tests, `go vet`, and a production image build. CD runs
only after a successful `main` CI or a manual dispatch; it never deploys
directly from `dev`.

GitHub `production` environment secrets:

```text
AZURE_CLIENT_ID
AZURE_TENANT_ID
AZURE_SUBSCRIPTION_ID
```

GitHub `production` environment variables:

```text
AZURE_RESOURCE_GROUP
AZURE_CONTAINER_REGISTRY
AZURE_CONTAINER_APP
AZURE_MIGRATION_JOB
```

## DNS

The live custom hostname is:

```text
https://tinyurl.haki-user.in
```

The DNS records are:

| Record | Value |
|---|---|
| `CNAME tinyurl.haki-user.in` | `tinyurl-api.ashyisland-4b5b4213.southeastasia.azurecontainerapps.io` |
| `TXT asuid.tinyurl.haki-user.in` | Container Apps custom domain verification id |

Container Apps binds the hostname with an Azure-managed certificate and SNI.
The application deployment sets
`TINYURL_BASE_URL=https://tinyurl.haki-user.in`, so newly created links return
the custom hostname.

## Operational Limitations

- Scale-to-zero creates cold-start latency.
- Redis is single-replica and disposable rather than managed or highly
  available.
- Authentication, rate limiting, cache metrics, tracing, and alerts remain
  backlog work.
- The single-region database is still the authoritative regional dependency.
