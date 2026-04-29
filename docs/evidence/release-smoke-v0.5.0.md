# Release Smoke v0.5.0

Date: 2026-04-29

Command:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\smoke-release.ps1 -Version v0.5.0
```

Proof:

```text
v0.5.0
scanned 4 facts from 3 sources
release smoke passed with C:\Users\garae\AppData\Local\Temp\ctx-release-smoke-v0.5.0\ctx_v0.5.0_windows_amd64\ctx.exe
checksum verified for ctx_v0.5.0_windows_amd64.zip
attestation verified for ctx_v0.5.0_windows_amd64.zip
```

Hosted proof:

- CI: https://github.com/oxhq/ctx/actions/runs/25125792235
- Dogfood: https://github.com/oxhq/ctx/actions/runs/25125792236
- Release: https://github.com/oxhq/ctx/actions/runs/25125826982
- Release assets: https://github.com/oxhq/ctx/releases/tag/v0.5.0

Boundary:

- This proves the published Windows amd64 archive can be downloaded, checksum-verified, provenance-verified with GitHub artifact attestations, and run through the CLI smoke path.
- This does not prove code-signed binaries or package-manager installation.
