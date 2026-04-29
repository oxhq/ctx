param(
    [string] $Version = "v0.2.0",
    [string] $Repo = "oxhq/ctx"
)

$ErrorActionPreference = "Stop"

$work = Join-Path $env:TEMP "ctx-release-smoke-$Version"
Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $work | Out-Null

gh release download $Version --repo $Repo --pattern "ctx_${Version}_windows_amd64.zip" --dir $work --clobber
gh release download $Version --repo $Repo --pattern "SHA256SUMS" --dir $work --clobber
$zip = Join-Path $work "ctx_${Version}_windows_amd64.zip"
$checksums = Join-Path $work "SHA256SUMS"
$expectedLine = Get-Content -LiteralPath $checksums | Where-Object { $_ -match [regex]::Escape((Split-Path -Leaf $zip)) } | Select-Object -First 1
if (-not $expectedLine) {
    throw "checksum entry not found for $(Split-Path -Leaf $zip)"
}
$expectedHash = ($expectedLine -split '\s+')[0].ToUpperInvariant()
$actualHash = (Get-FileHash -LiteralPath $zip -Algorithm SHA256).Hash.ToUpperInvariant()
if ($actualHash -ne $expectedHash) {
    throw "checksum mismatch for $(Split-Path -Leaf $zip): expected $expectedHash got $actualHash"
}
Expand-Archive -LiteralPath $zip -DestinationPath $work -Force
$ctx = Get-ChildItem -LiteralPath $work -Recurse -Filter ctx.exe | Select-Object -First 1
if (-not $ctx) {
    throw "ctx.exe not found in release archive"
}

$fixture = Join-Path $work "fixture"
New-Item -ItemType Directory -Force -Path $fixture | Out-Null
Set-Content -LiteralPath (Join-Path $fixture "go.mod") -Value "module example.com/fixture`n" -NoNewline
Set-Content -LiteralPath (Join-Path $fixture "planner.go") -Value "package fixture`nfunc TransformPlanner() {}`n" -NoNewline
Set-Content -LiteralPath (Join-Path $fixture "cases.jsonl") -Value '{"task":"refactor transform planner","expected_touched_areas":["planner.go"],"budget":300,"baseline_mode":"naive"}' -NoNewline

& $ctx.FullName version
& $ctx.FullName --help | Out-Null
& $ctx.FullName scan $fixture
& $ctx.FullName compile "refactor transform planner" --repo $fixture --budget 1200 --format markdown | Out-Null
& $ctx.FullName explain --repo $fixture --last | Out-Null
& $ctx.FullName bench --repo $fixture --cases (Join-Path $fixture "cases.jsonl") --baseline naive | Out-Null

Write-Output "release smoke passed with $($ctx.FullName)"
Write-Output "checksum verified for $(Split-Path -Leaf $zip)"
