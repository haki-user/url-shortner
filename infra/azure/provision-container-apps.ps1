param(
    [string]$ResourceGroup = "tinyurl-student-rg",
    [string]$Location = "southeastasia",
    [string]$RegistryName = "tinyurl7yycx7ze3vtdu",
    [string]$KeyVaultName = "tinyurl-kv-7yycx7ze3vtdu",
    [string]$CustomDomain = "tinyurl.haki-user.in",
    [string]$DiagnosticsToken = "",
    [Parameter(Mandatory = $true)]
    [string]$ImageTag
)

$ErrorActionPreference = "Stop"

$requiredProviders = @(
    "Microsoft.App",
    "Microsoft.ManagedIdentity"
)

foreach ($provider in $requiredProviders) {
    $state = az provider show `
        --namespace $provider `
        --query registrationState `
        --output tsv

    if ($state -ne "Registered") {
        Write-Host "Registering $provider..."
        az provider register --namespace $provider --wait --output none
    }
}

$deploymentJSON = az deployment group create `
    --name "tinyurl-container-apps" `
    --resource-group $ResourceGroup `
    --template-file "$PSScriptRoot\container-apps.bicep" `
    --parameters `
        location=$Location `
        registryName=$RegistryName `
        keyVaultName=$KeyVaultName `
        imageTag=$ImageTag `
        diagnosticsToken=$DiagnosticsToken `
    --query properties.outputs

if ($LASTEXITCODE -ne 0) {
    throw "Azure Container Apps deployment failed"
}

$deployment = $deploymentJSON | ConvertFrom-Json
$environmentName = $deployment.environmentName.value
$applicationName = $deployment.applicationName.value

if ($CustomDomain.Trim() -ne "") {
    $certificateId = az containerapp env certificate list `
        --resource-group $ResourceGroup `
        --name $environmentName `
        --query "[?properties.subjectName=='$CustomDomain'].id | [0]" `
        --output tsv

    if ($LASTEXITCODE -ne 0 -or $certificateId.Trim() -eq "") {
        throw "Managed certificate for $CustomDomain was not found in $environmentName"
    }

    az containerapp hostname add `
        --resource-group $ResourceGroup `
        --name $applicationName `
        --hostname $CustomDomain `
        --output none

    az containerapp hostname bind `
        --resource-group $ResourceGroup `
        --name $applicationName `
        --hostname $CustomDomain `
        --certificate $certificateId `
        --output none

    if ($LASTEXITCODE -ne 0) {
        throw "Failed to bind $CustomDomain to $applicationName"
    }
}

[pscustomobject]@{
    ResourceGroup       = $ResourceGroup
    Environment         = $environmentName
    Application         = $applicationName
    ApplicationURL      = $deployment.applicationURL.value
    PlatformURL         = $deployment.platformURL.value
    MigrationJob        = $deployment.migrationJobName.value
    ManagedIdentity     = $deployment.managedIdentityName.value
    RedisURL            = $deployment.redisURL.value
    MinimumReplicas     = 0
    RedirectCache       = "private utility VM Redis"
} | Format-List
