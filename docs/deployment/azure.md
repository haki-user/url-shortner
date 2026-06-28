# Azure VM Deployment

## Topology

The student-credit deployment is deliberately smaller than the target
multi-service architecture:

```text
GitHub Actions
    |
    | OIDC + immutable image
    v
Azure Container Registry
    |
    | VM managed identity pulls image
    v
Ubuntu B1s VM
|-- Caddy :80/:443
|-- linkd :8080, private
`-- Redis :6379, private and disposable
          |
          v
Azure PostgreSQL Flexible Server B1ms
private delegated subnet
```

Postgres is managed and authoritative. Redis runs on the VM with a 128 MB
memory limit, no persistence, and no host port. Losing the VM loses only
rebuildable cache data and the stateless application.

This topology is suitable for learning and a low-traffic demonstration. It is
not the target high-availability architecture because the VM is one failure
domain and one application replica.

## Student Benefit And Cost Guardrails

Azure for Students currently advertises, for eligible new accounts:

- 750 hours/month of B1s Linux VM usage for 12 months;
- 750 hours/month of PostgreSQL Flexible Server B1ms, 32 GB storage, and
  32 GB backup storage for 12 months;
- one Standard Azure Container Registry with 100 GB storage for 12 months.

Eligibility is subscription- and region-dependent. Free compute does not imply
that public IPv4, disks, bandwidth, DNS, or excess usage is free.

Before provisioning:

1. Confirm the three benefits in Azure Portal's **Free services** page.
2. Create a Cost Management budget alert.
3. Use `Standard_B1s`, `Standard_B1ms`, 32 GB PostgreSQL storage, and
   Standard LRS VM disk exactly as declared.
4. Check Cost Analysis after 24 hours.
5. Delete the resource group when the demonstration is no longer needed.

## Security Boundaries

- Only VM ports `80` and `443` are public.
- Port `22` is restricted to the administrator's current public CIDR.
- Redis is reachable only on the Docker bridge network.
- PostgreSQL uses a delegated private subnet and private DNS.
- PostgreSQL credentials are stored in Azure Key Vault.
- The VM accesses Key Vault and ACR with its system-assigned managed identity.
- GitHub accesses Azure through a federated OIDC identity, not a client secret.
- Caddy obtains and renews public TLS certificates after DNS points to the VM.

## Infrastructure

[`infra/azure/main.bicep`](../../infra/azure/main.bicep) declares:

- virtual network, VM subnet, and delegated PostgreSQL subnet;
- network security group;
- static public IP and Ubuntu B1s VM;
- PostgreSQL Flexible Server B1ms and `tinyurl` database;
- Standard ACR;
- Key Vault and database URL secret;
- VM managed-identity role assignments.

Required Bicep parameters:

```text
sshPublicKey
sshSourceCidr
postgresAdminPassword
```

Use a URL-safe PostgreSQL password because it is embedded in a connection URL.

## Runtime

[`deploy/azure-vm/compose.yaml`](../../deploy/azure-vm/compose.yaml) runs Caddy,
`linkd`, Redis, and an opt-in migration container.

[`deploy/azure-vm/deploy.sh`](../../deploy/azure-vm/deploy.sh):

1. logs into Azure using the VM identity;
2. authenticates Docker to ACR;
3. retrieves the database URL from Key Vault;
4. pulls the immutable Git SHA image;
5. runs migrations;
6. starts Redis, `linkd`, and Caddy;
7. waits for readiness.

## DNS

After infrastructure provisioning, create an `A` record:

```text
<chosen-subdomain> -> <vmPublicIP Bicep output>
```

Do not trigger the first CD run until public DNS resolves to that address.
Caddy needs public port 80 or 443 reachability to issue the certificate.

## CI/CD

CI runs on pull requests and pushes to `dev` and `main`:

```text
go test -race ./...
go vet ./...
production Docker build
```

The initial CD workflow is manual:

```text
GitHub OIDC login
build and push <registry>/tinyurl-linkd:<git-sha>
install runtime files through Azure VM Run Command
run migrations and deploy remotely
verify https://<domain>/readyz
```

After the first manual deployment succeeds, add a `push` trigger for `main`.

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
AZURE_VM_NAME
AZURE_KEY_VAULT
TINYURL_DOMAIN
```

Run
[`bootstrap-github-oidc.ps1`](../../infra/azure/bootstrap-github-oidc.ps1)
after infrastructure exists. It creates the federated identity and prints the
three IDs to configure in GitHub.

## Operational Limitations

- The VM is not highly available.
- Redis is not shared across replicas.
- OS and Docker patching remain our responsibility.
- VM disk and public IPv4 may consume credit.
- Application authentication still uses the development `X-Owner-ID` model.
- Cache metrics, circuit breaker, tracing, and alerting remain backlog work.
