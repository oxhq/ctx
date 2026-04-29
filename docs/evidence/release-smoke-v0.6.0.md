# Release Smoke v0.6.0

Date: 2026-04-29

Command:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\smoke-release.ps1 -Version v0.6.0
```

Proof:

```text
v0.6.0
scanned 4 facts from 3 sources
release smoke passed with C:\Users\garae\AppData\Local\Temp\ctx-release-smoke-v0.6.0\ctx_v0.6.0_windows_amd64\ctx.exe
checksum verified for ctx_v0.6.0_windows_amd64.zip
attestation verified for ctx_v0.6.0_windows_amd64.zip
```

Hosted proof:

- CI: https://github.com/oxhq/ctx/actions/runs/25126254488
- Dogfood: https://github.com/oxhq/ctx/actions/runs/25126254505
- Release: https://github.com/oxhq/ctx/actions/runs/25126290719
- Authenticode preflight: https://github.com/oxhq/ctx/actions/runs/25126414184
- Release assets: https://github.com/oxhq/ctx/releases/tag/v0.6.0

Authenticode preflight:

```text
Preflight passed for release asset, checksum, attestation, and archive layout.
Signing configuration still missing: AZURE_CLIENT_ID, AZURE_TENANT_ID, AZURE_SUBSCRIPTION_ID, AZURE_SIGNING_ENDPOINT, AZURE_SIGNING_ACCOUNT, AZURE_SIGNING_CERTIFICATE_PROFILE
```

Boundary:

- This proves the published Windows amd64 archive can be downloaded, checksum-verified, provenance-verified with GitHub artifact attestations, and run through the CLI smoke path.
- This proves the manual Authenticode workflow preflight can validate the release asset path without signing credentials.
- This does not prove Authenticode signing. The repository still needs the Azure Artifact Signing secrets and variables before `mode=sign` can publish `ctx_v0.6.0_windows_amd64_signed.zip`.
