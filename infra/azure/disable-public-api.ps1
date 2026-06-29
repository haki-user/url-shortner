param(
    [string]$ResourceGroup = "tinyurl-student-rg",
    [string]$ContainerAppName = "tinyurl-api",
    [string]$UtilityVmName = "tinyurl-utility-vm",
    [switch]$DeallocateUtilityVm
)

$ErrorActionPreference = "Stop"

az containerapp ingress disable `
    --resource-group $ResourceGroup `
    --name $ContainerAppName `
    --output none

az containerapp update `
    --resource-group $ResourceGroup `
    --name $ContainerAppName `
    --min-replicas 0 `
    --max-replicas 1 `
    --output none

if ($DeallocateUtilityVm) {
    az vm deallocate `
        --resource-group $ResourceGroup `
        --name $UtilityVmName `
        --output none
}

[pscustomobject]@{
    ContainerApp = $ContainerAppName
    PublicIngress = "disabled"
    MinReplicas = 0
    MaxReplicas = 1
    UtilityVm = if ($DeallocateUtilityVm) { "deallocated" } else { "unchanged" }
} | Format-List
