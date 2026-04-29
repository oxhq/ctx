param(
    [string] $MorfxRepo = "C:\Users\garae\OneDrive\Documentos\GitHub\morfx",
    [string] $Cases = "benchmarks\morfx\cases.jsonl",
    [string] $Output = "docs\evidence\morfx-benchmark.latest.json"
)

$ErrorActionPreference = "Stop"

$root = Resolve-Path -LiteralPath (Join-Path $PSScriptRoot "..")
$morfx = Resolve-Path -LiteralPath $MorfxRepo
$casesPath = Resolve-Path -LiteralPath (Join-Path $root $Cases)
$outputPath = Join-Path $root $Output
$outputDir = Split-Path -Parent $outputPath

New-Item -ItemType Directory -Force -Path $outputDir | Out-Null

Push-Location $root
try {
    $result = go run ./cmd/ctx bench --repo $morfx.Path --cases $casesPath.Path --baseline naive
    if ($LASTEXITCODE -ne 0) {
        exit $LASTEXITCODE
    }
    $result | Set-Content -LiteralPath $outputPath -Encoding utf8
    $parsed = $result | ConvertFrom-Json
    $failed = @($parsed.cases | Where-Object { -not $_.expected_area_hit -or $_.token_reduction_percent -lt 30 })
    if ($failed.Count -gt 0) {
        Write-Error "Dogfood benchmark failed threshold checks"
    }
    Write-Output "wrote $outputPath"
}
finally {
    Pop-Location
}
