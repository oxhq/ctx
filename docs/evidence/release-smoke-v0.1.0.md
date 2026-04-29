# v0.1.0 Windows Release Smoke

Independent worker smoke-tested `ctx_v0.1.0_windows_amd64.zip` from GitHub Releases on Windows.

Proof:

- Downloaded `ctx_v0.1.0_windows_amd64.zip` from `oxhq/ctx`.
- Extracted `ctx.exe`.
- Ran `ctx.exe version` and got `v0.1.0`.
- Ran `scan`, `compile --format json --explain`, `explain --last`, and `bench` on a tiny Go fixture.
- The fixture benchmark hit the expected area. Token reduction was negative on the tiny fixture because compiled context included project facts; that is not used as the real quality benchmark.

Observed v0.1.0 UX gap:

- `ctx.exe --help` returned `unknown command "--help"`.
- v0.2 adds a small help surface.
