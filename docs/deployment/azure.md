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
          | private VNet + private DNS
          v
Azure PostgreSQL Flexible Server B1ms
```

PostgreSQL remains authoritative. The deployed API uses PostgreSQL directly
and sets `TINYURL_CACHE=none`; Redis remains an implemented architecture option
for sustained redirect load, but a continuously running Redis instance is not
cost-effective for this low-traffic demonstration.

## Cost Model

Container Apps Consumption can scale the API to zero replicas. The environment
uses platform-managed ingress, so this deployment does not create a dedicated
VM disk or Azure Public IP resource.

The retired VM topology incurred separate charges for:

- a Standard static public IPv4 address, billed until the IP resource is
  deleted even when detached;
- a provisioned Standard HDD OS disk;
- compute beyond any applicable student allowance.

PostgreSQL, ACR, Key Vault, network usage, and usage beyond Container Apps
grants can still consume student credit. Check Cost Analysis and keep a budget
alert enabled.

## Security Boundaries

- Container Apps terminates HTTPS and exposes only the application ingress.
- PostgreSQL remains in its delegated private subnet.
- Container Apps reaches PostgreSQL through a dedicated delegated `/27`
  subnet and the existing private DNS zone.
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

The app uses `minReplicas: 0` and `maxReplicas: 3`. Zero minimizes idle cost
but introduces cold-start latency after an idle period. Set the minimum to one
when latency matters more than idle cost.

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

The platform hostname is immediately available over HTTPS:

```text
https://tinyurl-api.<environment>.<region>.azurecontainerapps.io
```

For a custom hostname, create the DNS records requested by Container Apps,
bind the hostname, and use a free managed certificate. Then update
`TINYURL_BASE_URL` on the container app so newly created links return the
custom hostname.

## Operational Limitations

- Scale-to-zero creates cold-start latency.
- Redis caching is disabled in this deployment.
- Authentication, rate limiting, cache metrics, tracing, and alerts remain
  backlog work.
- The single-region database is still the authoritative regional dependency.
