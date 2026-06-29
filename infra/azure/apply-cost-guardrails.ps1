param(
    [string]$ResourceGroup = "tinyurl-student-rg",
    [string]$ContainerAppName = "tinyurl-api"
)

$ErrorActionPreference = "Stop"

az containerapp update `
    --resource-group $ResourceGroup `
    --name $ContainerAppName `
    --min-replicas 0 `
    --max-replicas 1 `
    --output none

[pscustomobject]@{
    ContainerApp = $ContainerAppName
    MinReplicas  = 0
    MaxReplicas  = 1
    PublicIngress = "unchanged"
} | Format-List
