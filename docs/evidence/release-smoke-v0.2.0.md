# v0.2.0 Windows Release Smoke

Smoke-tested `ctx_v0.2.0_windows_amd64.zip` from GitHub Releases on Windows.

Proof:

- Downloaded `ctx_v0.2.0_windows_amd64.zip` and `SHA256SUMS` from `oxhq/ctx`.
- Verified the archive SHA-256 hash against `SHA256SUMS`.
- Extracted `ctx.exe`.
- Ran `ctx.exe version` and got `v0.2.0`.
- Ran `ctx.exe --help`.
- Ran `scan`, `compile --format markdown`, `explain --last`, and `bench` on a tiny Go fixture.

Boundary:

- This proves checksum-verifiable Windows archive execution. It does not prove code signing or package-manager installation.
