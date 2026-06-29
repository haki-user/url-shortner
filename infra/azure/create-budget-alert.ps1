param(
    [string]$ResourceGroup = "tinyurl-student-rg",
    [string]$BudgetName = "tinyurl-student-monthly-5usd",
    [decimal]$AmountUsd = 5,
    [string]$ContactEmail = "Adityapratapsingh33@outlook.com",
    [string]$StartDate = "2026-06-01T00:00:00Z",
    [string]$EndDate = "2027-06-30T00:00:00Z"
)

$ErrorActionPreference = "Stop"

$subscriptionId = az account show --query id --output tsv
$scope = "/subscriptions/$subscriptionId/resourceGroups/$ResourceGroup"
$uri = "https://management.azure.com$scope/providers/Microsoft.Consumption/budgets/${BudgetName}?api-version=2023-11-01"

$notifications = @{}
foreach ($threshold in @(50, 80, 100)) {
    $notifications["actual_GreaterThan_${threshold}_Percent"] = @{
        enabled = $true
        operator = "GreaterThan"
        threshold = $threshold
        thresholdType = "Actual"
        locale = "en-us"
        contactEmails = @($ContactEmail)
        contactGroups = @()
        contactRoles = @()
    }
}

$body = @{
    properties = @{
        category = "Cost"
        amount = $AmountUsd
        timeGrain = "Monthly"
        timePeriod = @{
            startDate = $StartDate
            endDate = $EndDate
        }
        notifications = $notifications
    }
} | ConvertTo-Json -Depth 10

$tempBody = New-TemporaryFile
try {
    Set-Content -Path $tempBody -Value $body -NoNewline

    az rest `
        --method put `
        --uri $uri `
        --body "@$tempBody" `
        --headers Content-Type=application/json `
        --query "properties.{amount:amount,timeGrain:timeGrain,notifications:notifications}" `
        --output json
}
finally {
    Remove-Item $tempBody -Force
}
