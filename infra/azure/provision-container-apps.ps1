param(
    [string]$ResourceGroup = "tinyurl-student-rg",
    [string]$Location = "southeastasia",
    [string]$RegistryName = "tinyurl7yycx7ze3vtdu",
    [string]$KeyVaultName = "tinyurl-kv-7yycx7ze3vtdu",
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
    --query properties.outputs

if ($LASTEXITCODE -ne 0) {
    throw "Azure Container Apps deployment failed"
}

$deployment = $deploymentJSON | ConvertFrom-Json

[pscustomobject]@{
    ResourceGroup       = $ResourceGroup
    Environment         = $deployment.environmentName.value
    Application         = $deployment.applicationName.value
    ApplicationURL      = $deployment.applicationURL.value
    MigrationJob        = $deployment.migrationJobName.value
    ManagedIdentity     = $deployment.managedIdentityName.value
    MinimumReplicas     = 0
    RedirectCache       = "disabled in this cost-optimized deployment"
} | Format-List
