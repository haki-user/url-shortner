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

[`infra/azure/configure-utility-vm.ps1`](../../infra/azure/configure-utility-vm.ps1)
configures software inside the utility VM through Azure Run Command:

- Redis server with no persistence, 128 MB max memory, and `allkeys-lru`;
- Tailscale client and `tailscaled`;
- IPv4/IPv6 forwarding for later subnet-router use.

Tailscale is installed but not joined to a tailnet by default. Joining requires
a real auth key or interactive browser login:

```bash
sudo tailscale up --ssh --advertise-routes=10.20.0.0/16
```

If using an auth key, pass it only inside a trusted shell session and do not
commit it:

```bash
sudo tailscale up --auth-key <tailscale-auth-key> --ssh --advertise-routes=10.20.0.0/16
```

Approve the advertised route in the Tailscale admin console before expecting
laptop-to-VNet connectivity.

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
| API maximum replicas | One; strict student cost guardrail | Load-tested limit with a database connection budget |
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

## Cost Guardrails

The public API can receive arbitrary internet traffic. Azure budgets are
alerts, not hard stops, so the strongest live guardrail is keeping scale-out
small and having a fast kill switch.

Normal cost-guarded public mode:

```powershell
$env:Path = "C:\Program Files\Microsoft SDKs\Azure\CLI2\wbin;$env:Path"

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\infra\azure\apply-cost-guardrails.ps1
```

This keeps:

```text
tinyurl-api minReplicas = 0
tinyurl-api maxReplicas = 1
```

Create or refresh the monthly budget alert:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\infra\azure\create-budget-alert.ps1 `
  -AmountUsd 5 `
  -ContactEmail "Adityapratapsingh33@outlook.com"
```

The budget is scoped to `tinyurl-student-rg` and sends email at 50%, 80%, and
100% actual spend. It warns; it does not stop Azure resources.

Emergency mode, disable the public API but keep Redis/VM running:

```powershell
$env:Path = "C:\Program Files\Microsoft SDKs\Azure\CLI2\wbin;$env:Path"

powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\infra\azure\disable-public-api.ps1
```

Deeper emergency mode, disable the public API and deallocate the utility VM:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\infra\azure\disable-public-api.ps1 `
  -DeallocateUtilityVm
```

Restore public API:

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass `
  -File .\infra\azure\restore-public-api.ps1
```

Current hard cost boundaries:

- no public IP resource;
- no Azure Bastion;
- no Azure VPN Gateway;
- no Redis Container App;
- public API scales from zero to at most one replica;
- Redis memory is capped at 128 MB;
- VM compute uses `Standard_B2ats_v2`, which is Azure for Students
  free-service eligible for 750 hours/month.

Remaining cost exposure:

- the utility VM OS disk;
- PostgreSQL/ACR/Key Vault/private DNS;
- public Container Apps request/compute/bandwidth while ingress is enabled;
- VM compute if the free-service allowance is exhausted.

## Tailscale And DBeaver

Tailscale is installed on `tinyurl-utility-vm`, but it is not joined to a
tailnet until you run `tailscale up`.

Create a Tailscale auth key:

1. Open Tailscale admin console.
2. Go to **Settings -> Keys -> Generate auth key**.
3. Use an ephemeral or reusable key for this VM.
4. Treat the key like a password. Do not commit it.

Join the utility VM to Tailscale and advertise the Azure VNet route:

```powershell
$env:TS_AUTH_KEY = "<paste-auth-key-here>"

$script = @"
sudo tailscale up `
  --auth-key $env:TS_AUTH_KEY `
  --ssh `
  --advertise-routes=10.20.0.0/16
"@

$tmp = New-TemporaryFile
Set-Content -Path $tmp -Value $script -NoNewline

az vm run-command invoke `
  --resource-group tinyurl-student-rg `
  --name tinyurl-utility-vm `
  --command-id RunShellScript `
  --scripts "@$tmp" `
  --query "value[].message" `
  --output tsv

Remove-Item $tmp -Force
Remove-Item Env:\TS_AUTH_KEY
```

Then approve the route in Tailscale:

```text
Tailscale admin console
-> Machines
-> tinyurl-utility-vm
-> Subnet routes
-> approve 10.20.0.0/16
```

Install Tailscale on your laptop and connect to the same tailnet. After route
approval, your laptop should be able to reach private Azure addresses such as
`10.20.4.4`.

Optional connectivity checks from your laptop:

```powershell
tailscale status
Test-NetConnection 10.20.4.4 -Port 6379
```

Get the Postgres password from Key Vault. If your user has not been granted
secret access yet, first grant yourself `Key Vault Secrets Officer`:

```powershell
$vaultId = az keyvault show `
  -g tinyurl-student-rg `
  -n tinyurl-kv-7yycx7ze3vtdu `
  --query id `
  -o tsv

az role assignment create `
  --assignee dc5f3d67-039b-48bd-83e1-da027ef4514e `
  --role "Key Vault Secrets Officer" `
  --scope $vaultId
```

Read the database URL:

```powershell
az keyvault secret show `
  --vault-name tinyurl-kv-7yycx7ze3vtdu `
  --name tinyurl-database-url `
  --query value `
  -o tsv
```

DBeaver settings when Tailscale subnet routing is approved:

```text
Driver: PostgreSQL
Host: tinyurl-pg-7yycx7ze3vtdu.postgres.database.azure.com
Port: 5432
Database: tinyurl
Username: tinyurladmin
Password: value from Key Vault connection string
SSL mode: require
```

If DNS does not resolve from your laptop through Tailscale, use the private IP
resolved from the VM:

```powershell
$script = "getent hosts tinyurl-pg-7yycx7ze3vtdu.postgres.database.azure.com"
$tmp = New-TemporaryFile
Set-Content -Path $tmp -Value $script -NoNewline

az vm run-command invoke `
  -g tinyurl-student-rg `
  -n tinyurl-utility-vm `
  --command-id RunShellScript `
  --scripts "@$tmp" `
  --query "value[].message" `
  -o tsv

Remove-Item $tmp -Force
```

Then put that private IP in DBeaver host and keep SSL mode as `require`.
