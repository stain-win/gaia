package tui

// Daemon state constants
const (
	DaemonStatusLocked   = "locked"
	DaemonStatusUnlocked = "unlocked"
	DaemonStatusOffline  = "offline"
	DaemonStatusStopped  = "stopped"
	DaemonStatusStarting = "starting"
)

// isOffline checks if daemon is in any offline-like state
func isOffline(status string) bool {
	return status == DaemonStatusOffline ||
		status == DaemonStatusStopped ||
		status == DaemonStatusStarting
}

// isDaemonAccessible checks if daemon is in a state where data can be accessed
func isDaemonAccessible(status string) bool {
	return status == DaemonStatusUnlocked
}

// isDaemonLocked checks if daemon is locked
func isDaemonLocked(status string) bool {
	return status == DaemonStatusLocked
}
