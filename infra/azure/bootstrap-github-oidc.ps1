param(
    [Parameter(Mandatory = $true)]
    [string]$ResourceGroup,

    [string]$GitHubOwner = "haki-user",
    [string]$GitHubRepository = "url-shortner",
    [string]$Environment = "production",
    [string]$ApplicationName = "tinyurl-github-deploy"
)

$ErrorActionPreference = "Stop"

$subscription = az account show | ConvertFrom-Json
$resourceGroupId = az group show --name $ResourceGroup --query id --output tsv

$application = az ad app create --display-name $ApplicationName | ConvertFrom-Json
$servicePrincipal = az ad sp create --id $application.appId | ConvertFrom-Json

$credential = @{
    name      = "github-$Environment"
    issuer    = "https://token.actions.githubusercontent.com"
    subject   = "repo:${GitHubOwner}/${GitHubRepository}:environment:${Environment}"
    audiences = @("api://AzureADTokenExchange")
} | ConvertTo-Json -Compress

$credentialFile = New-TemporaryFile
try {
    Set-Content -LiteralPath $credentialFile -Value $credential -NoNewline
    az ad app federated-credential create `
        --id $application.appId `
        --parameters "@$credentialFile" `
        --output none
}
finally {
    Remove-Item -LiteralPath $credentialFile -Force
}

az role assignment create `
    --assignee-object-id $servicePrincipal.id `
    --assignee-principal-type ServicePrincipal `
    --role Contributor `
    --scope $resourceGroupId `
    --output none

$registryId = az acr show `
    --resource-group $ResourceGroup `
    --name (az acr list --resource-group $ResourceGroup --query "[0].name" --output tsv) `
    --query id `
    --output tsv

az role assignment create `
    --assignee-object-id $servicePrincipal.id `
    --assignee-principal-type ServicePrincipal `
    --role AcrPush `
    --scope $registryId `
    --output none

[pscustomobject]@{
    AZURE_CLIENT_ID       = $application.appId
    AZURE_TENANT_ID       = $subscription.tenantId
    AZURE_SUBSCRIPTION_ID = $subscription.id
    OIDC_SUBJECT          = "repo:${GitHubOwner}/${GitHubRepository}:environment:${Environment}"
} | Format-List
