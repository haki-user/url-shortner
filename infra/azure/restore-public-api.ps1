param(
    [string]$ResourceGroup = "tinyurl-student-rg",
    [string]$ContainerAppName = "tinyurl-api",
    [string]$UtilityVmName = "tinyurl-utility-vm",
    [int]$TargetPort = 8080
)

$ErrorActionPreference = "Stop"

az vm start `
    --resource-group $ResourceGroup `
    --name $UtilityVmName `
    --output none

az containerapp ingress enable `
    --resource-group $ResourceGroup `
    --name $ContainerAppName `
    --type external `
    --target-port $TargetPort `
    --transport auto `
    --allow-insecure false `
    --output none

az containerapp update `
    --resource-group $ResourceGroup `
    --name $ContainerAppName `
    --min-replicas 0 `
    --max-replicas 1 `
    --output none

[pscustomobject]@{
    ContainerApp = $ContainerAppName
    PublicIngress = "enabled"
    MinReplicas = 0
    MaxReplicas = 1
    UtilityVm = "started"
} | Format-List
