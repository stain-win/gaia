package tui

import (
	"time"
)
import tea "github.com/charmbracelet/bubbletea"

// BackMsg is a custom message to signal returning to the previous menu.
type BackMsg struct{}

// ListNamespacesMsg is a message to trigger fetching namespaces from the daemon.
type ListNamespacesMsg struct{}

// NamespacesReadyMsg is a message for when namespaces are fetched.
type NamespacesReadyMsg []string

// A mock function to simulate fetching namespaces from the daemon.
func mockListNamespaces() tea.Cmd {
	return func() tea.Msg {
		time.Sleep(50 * time.Millisecond) // Simulate network latency
		namespaces := []string{"common", "client-a", "client-b"}
		return NamespacesReadyMsg(namespaces)
	}
}

func addRecordToDaemon(namespace, key, value string) tea.Cmd {
	return func() tea.Msg {
		// This is a placeholder; in a real app, you'd make a gRPC call.
		// Assume success for now
		return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
			return nil // Using Tick to simulate an async operation that completes
		})
	}
}
