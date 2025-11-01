feat(tui): fix register client, add lock status indicator, improve navigation

## Summary

This commit completes the TUI cleanup with three major improvements:

1. Fixed Register Client functionality
2. Added visual lock/unlock status indicator
3. Improved back navigation with context-aware key handling

## Changes

### Fixed Register Client (Certificate Management)
- Implemented full gRPC RegisterClient flow
- Generate and save client certificates
- Show success/error feedback in status bar
- Files: tui/messages.go, tui/update.go, tui/register_client_form.go

### Added Lock Status Indicator
- Status bar now shows daemon state with color-coded icons:
  * 🔓 unlocked (green) - Ready to use
  * 🔒 locked (orange) - Needs unlock
  * ⚠️ offline (red) - Cannot connect
- Always visible at top of screen
- Files: tui/view.go

### Improved Back Navigation
- Added 'b' key for back in menus (intuitive)
- Keep 'esc' for forms only (no conflict with text input)
- Context-aware: menus use b/esc, forms use esc only
- Files: tui/keys.go, tui/update.go, tui/register_client_form.go

### Enhanced Status Messages
- All operations show feedback in status bar
- Success messages with ✓ checkmark
- Error messages with ❌ cross
- Professional, in-app feedback (no console output)
- Files: tui/update.go, tui/view.go

## Testing

✅ Build successful
✅ go vet clean
✅ All functionality verified
✅ No breaking changes

## Documentation

- TUI_CLEANUP_REPORT.md - Complete technical report
- TUI_CLEANUP.md - Detailed documentation
- TUI_CLEANUP_QUICK_REF.md - Quick reference
- TUI_CLEANUP_VISUAL.md - Visual comparison
- test_tui_changes.sh - Test script

## Files Modified

```
apps/gaia/tui/keys.go
apps/gaia/tui/messages.go
apps/gaia/tui/register_client_form.go
apps/gaia/tui/update.go
apps/gaia/tui/view.go
```

## Migration

No breaking changes. Fully backward compatible.

