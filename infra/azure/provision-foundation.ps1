param(
    [string]$ResourceGroup = "tinyurl-student-rg",
    [string]$Location = "southeastasia",
    [string]$Prefix = "tinyurl"
)

$ErrorActionPreference = "Stop"

$randomBytes = [byte[]]::new(24)
$randomNumberGenerator = [System.Security.Cryptography.RandomNumberGenerator]::Create()
try {
    $randomNumberGenerator.GetBytes($randomBytes)
}
finally {
    $randomNumberGenerator.Dispose()
}
$postgresPassword = (
    [Convert]::ToBase64String($randomBytes).
        TrimEnd("=").
        Replace("+", "A").
        Replace("/", "b")
) + "Aa9"

$requiredProviders = @(
    "Microsoft.Authorization",
    "Microsoft.ContainerRegistry",
    "Microsoft.DBforPostgreSQL",
    "Microsoft.KeyVault",
    "Microsoft.Network"
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

$resourceGroupExists = az group exists --name $ResourceGroup
if ($resourceGroupExists -eq "false") {
    az group create `
        --name $ResourceGroup `
        --location $Location `
        --tags project=tinyurl environment=student-demo `
        --output none

    if ($LASTEXITCODE -ne 0) {
        throw "Azure resource group creation failed"
    }
}

$existingPostgresCount = az postgres flexible-server list `
    --resource-group $ResourceGroup `
    --query "length(@)" `
    --output tsv

if ([int]$existingPostgresCount -gt 0) {
    throw (
        "The TinyURL foundation already exists in '$ResourceGroup'. " +
        "Refusing to rotate its PostgreSQL password; use " +
        "provision-container-apps.ps1 for application infrastructure."
    )
}

$parameters = @{
    '$schema'      = "https://schema.management.azure.com/schemas/2019-04-01/deploymentParameters.json#"
    contentVersion = "1.0.0.0"
    parameters     = @{
        location                  = @{ value = $Location }
        prefix                    = @{ value = $Prefix }
        postgresAdminPassword     = @{ value = $postgresPassword }
    }
}

$parameterFile = New-TemporaryFile
try {
    $parameters |
        ConvertTo-Json -Depth 10 |
        Set-Content -LiteralPath $parameterFile -NoNewline

    $deploymentJSON = az deployment group create `
        --name "tinyurl-infrastructure" `
        --resource-group $ResourceGroup `
        --template-file "$PSScriptRoot\main.bicep" `
        --parameters "@$parameterFile" `
        --query properties.outputs

    if ($LASTEXITCODE -ne 0) {
        throw "Azure infrastructure deployment failed"
    }

    $deployment = $deploymentJSON | ConvertFrom-Json
}
finally {
    Remove-Item -LiteralPath $parameterFile -Force
    $postgresPassword = $null
}

[pscustomobject]@{
    ResourceGroup           = $ResourceGroup
    Location                = $Location
    ContainerRegistry       = $deployment.registryName.value
    ContainerRegistryServer = $deployment.registryLoginServer.value
    KeyVault                = $deployment.keyVaultName.value
    PostgresServer          = $deployment.postgresServerName.value
    PostgresHost            = $deployment.postgresHost.value
} | Format-List
