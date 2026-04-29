param(
    [string] $Version = "v0.5.0",
    [string] $Repo = "oxhq/ctx"
)

$ErrorActionPreference = "Stop"

$work = Join-Path $env:TEMP "ctx-signed-release-smoke-$Version"
Remove-Item -LiteralPath $work -Recurse -Force -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $work | Out-Null

$archiveName = "ctx_${Version}_windows_amd64_signed.zip"
gh release download $Version --repo $Repo --pattern $archiveName --dir $work --clobber
gh release download $Version --repo $Repo --pattern "SHA256SUMS.signed" --dir $work --clobber

$zip = Join-Path $work $archiveName
$checksums = Join-Path $work "SHA256SUMS.signed"
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
    throw "ctx.exe not found in signed release archive"
}

$signature = Get-AuthenticodeSignature -LiteralPath $ctx.FullName
if ($signature.Status -ne "Valid") {
    throw "Authenticode signature is not valid: $($signature.Status)"
}

& $ctx.FullName version
& $ctx.FullName --help | Out-Null

Write-Output "signed release smoke passed with $($ctx.FullName)"
Write-Output "checksum verified for $archiveName"
Write-Output "Authenticode signature verified for ctx.exe"
