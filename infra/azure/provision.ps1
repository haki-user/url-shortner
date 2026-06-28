param(
    [string]$ResourceGroup = "tinyurl-student-rg",
    [string]$Location = "centralindia",
    [string]$Prefix = "tinyurl",
    [string]$VmSize = "Standard_B1s",
    [string]$SshPublicKeyPath = "$HOME\.ssh\id_ed25519.pub",
    [string]$SshSourceCidr = ""
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path -LiteralPath $SshPublicKeyPath)) {
    throw "SSH public key not found at $SshPublicKeyPath"
}

if ([string]::IsNullOrWhiteSpace($SshSourceCidr)) {
    $publicIP = (
        Invoke-RestMethod -Uri "https://api.ipify.org" -TimeoutSec 10
    ).Trim()
    $SshSourceCidr = "$publicIP/32"
}

$sshPublicKey = (Get-Content -LiteralPath $SshPublicKeyPath -Raw).Trim()

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
    "Microsoft.Compute",
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

$parameters = @{
    '$schema'      = "https://schema.management.azure.com/schemas/2019-04-01/deploymentParameters.json#"
    contentVersion = "1.0.0.0"
    parameters     = @{
        location                  = @{ value = $Location }
        prefix                    = @{ value = $Prefix }
        vmSize                    = @{ value = $VmSize }
        sshPublicKey              = @{ value = $sshPublicKey }
        sshSourceCidr             = @{ value = $SshSourceCidr }
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
    VmName                  = $deployment.vmName.value
    VmSize                  = $VmSize
    VmPublicIP              = $deployment.vmPublicIP.value
    ContainerRegistry       = $deployment.registryName.value
    ContainerRegistryServer = $deployment.registryLoginServer.value
    KeyVault                = $deployment.keyVaultName.value
    PostgresServer          = $deployment.postgresServerName.value
    PostgresHost            = $deployment.postgresHost.value
    SshSourceCidr           = $SshSourceCidr
} | Format-List
