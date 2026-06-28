# Azure Deployment Plan

## Cost-Aware First Release

Use:

```text
GitHub Actions CI
        |
        v
Azure Container Registry
        |
        +-> Container Apps Job: /app/migrate
        |
        `-> Container App: /app/linkd
                    |
                    +-> Azure Database for PostgreSQL Flexible Server
                    `-> cache disabled for the first release
```

Azure Container Apps Consumption can scale to zero and includes a monthly free
grant. PostgreSQL eligibility and cost depend on the subscription offer,
region, compute tier, storage, and whether the free-account database benefit is
still available.

The first release sets:

```text
TINYURL_CACHE=none
```

This keeps the system correct because Postgres remains the source of truth.
It also avoids creating a paid Redis dependency merely for a demo.

Do not create a new legacy Azure Cache for Redis resource. That service is on a
retirement path. A later production-grade cache should use Azure Managed Redis
after reviewing its current price. For a temporary demonstration, an
ephemeral Redis sidecar is possible, but it is replica-local and is not the
shared regional cache in the target architecture.

## CI Then CD

The checked-in CI workflow runs on pull requests and pushes to `dev` or `main`:

```text
go test -race ./...
go vet ./...
docker build production image
```

Add CD after the first manual Azure release establishes the real resource
names. CD will:

```text
authenticate to Azure through GitHub OIDC
build and push image tagged with Git SHA
update the manual migration job to that image
run migration job and wait for success
update the Container App revision
wait for readiness
```

Use OIDC rather than storing an Azure client secret in GitHub.

## Required Azure Resources

- One resource group.
- One Container Apps environment using the Consumption profile.
- One Azure Container Registry.
- One PostgreSQL Flexible Server and database.
- One manual Container Apps migration job.
- One externally accessible Container App for `linkd`.
- One Microsoft Entra application or user-assigned managed identity configured
  with GitHub federated credentials.

Start with minimum practical development sizes and configure a budget alert
before provisioning.

## Required GitHub Configuration

OIDC secrets:

```text
AZURE_CLIENT_ID
AZURE_TENANT_ID
AZURE_SUBSCRIPTION_ID
```

Repository or environment variables:

```text
AZURE_RESOURCE_GROUP
AZURE_CONTAINER_APP
AZURE_MIGRATION_JOB
AZURE_CONTAINER_REGISTRY
```

Database credentials belong in Azure Container Apps secrets, not GitHub
workflow files.

## Local Prerequisites

The workstation currently does not have Azure CLI or GitHub CLI installed.
Before provisioning:

```text
install Azure CLI
az login
az account set --subscription <subscription>
install the Azure Container Apps CLI extension
```

The first provisioning pass should remain manual and reviewed. Once its
commands and cost are known-good, encode them as Bicep and enable CD.
