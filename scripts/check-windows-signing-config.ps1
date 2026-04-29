param(
    [string] $Repo = "oxhq/ctx"
)

$ErrorActionPreference = "Stop"

$requiredSecrets = @(
    "AZURE_CLIENT_ID",
    "AZURE_TENANT_ID",
    "AZURE_SUBSCRIPTION_ID"
)

$requiredVariables = @(
    "AZURE_SIGNING_ENDPOINT",
    "AZURE_SIGNING_ACCOUNT",
    "AZURE_SIGNING_CERTIFICATE_PROFILE"
)

function Get-GhNames {
    param(
        [string] $Kind
    )

    $json = & gh $Kind list --repo $Repo --json name
    if (-not $json) {
        return @()
    }
    return @($json | ConvertFrom-Json | ForEach-Object { $_.name })
}

$secretNames = Get-GhNames -Kind "secret"
$variableNames = Get-GhNames -Kind "variable"

$missingSecrets = @($requiredSecrets | Where-Object { $secretNames -notcontains $_ })
$missingVariables = @($requiredVariables | Where-Object { $variableNames -notcontains $_ })

if ($missingSecrets.Count -eq 0 -and $missingVariables.Count -eq 0) {
    Write-Output "windows signing configuration present for $Repo"
    exit 0
}

if ($missingSecrets.Count -gt 0) {
    Write-Output "missing repository secrets: $($missingSecrets -join ', ')"
}

if ($missingVariables.Count -gt 0) {
    Write-Output "missing repository variables: $($missingVariables -join ', ')"
}

exit 1
