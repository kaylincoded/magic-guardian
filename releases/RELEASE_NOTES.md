## What's New

### Bug Fixes
- **Fix false restock notifications**: The bot now correctly shifts its internal shop inventory when an item completely sells out, preventing subsequent stock updates from misaligning and firing phantom restocks.
- **Fix phantom restock for new items**: When the game adds new items to a shop, they no longer falsely appear as 0→N restocks.
- **Fix Android DNS resolution**: Removed `netgo` build tag so the Go binary uses Android's native DNS resolver.
- **Fix Android binary crash (SIGILL)**: Go binary correctly cross-compiled as PIE executable with `GOOS=android`.
- **Fix status bar overlap**: CSS `safe-area-inset-top` keeps header below Android system bar.
- **Fix port conflict**: Web UI server exits immediately if the port is already in use.
- **Fix header spacing**: Logo-to-title and status indicator gaps render correctly at all screen densities.

### Test Suite (112 tests)
- Full unit + integration tests across 5 packages
- Race detector clean
- Outcome-based assertions (verify payload field values, not just counts)
- Regression tests for both reported bugs

### CI/CD Pipeline
- GitHub Actions CI: test + build on every push/PR
- GitHub Actions Release: automated builds for all 7 platforms on version tags
- All actions updated to latest versions (checkout v6, setup-go v6, upload-artifact v7)

### UI
- shadcn green preset dark theme across all UI elements
- Flux-style toasts with happy/sad mac icons

### Downloads

| Platform | File |
|----------|------|
| Linux x86_64 | `magic-guardian-linux-amd64` |
| Linux ARM64 | `magic-guardian-linux-arm64` |
| macOS Intel | `magic-guardian-darwin-amd64` |
| macOS Apple Silicon | `magic-guardian-darwin-arm64` |
| Windows x86_64 | `magic-guardian-windows-amd64.exe` |
| Android ARM64 (bare binary) | `magic-guardian-android-arm64` |
| Android APK | `magic-guardian-android.apk` |

**Full Changelog**: https://github.com/kaylincoded/magic-guardian/compare/v0.2.0...v0.2.1
