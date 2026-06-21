package tui

import (
	"strings"
	"testing"
)

// TestSplashConverges verifies the spring-driven splash eventually settles and
// signals completion, fully revealing the logo along the way.
func TestSplashConverges(t *testing.T) {
	s := newSplashModel()
	s.width = 120
	s.height = 40

	if s.totalCols <= 0 {
		t.Fatalf("expected positive totalCols, got %d", s.totalCols)
	}

	const maxFrames = 1000 // generous cap (~16s at 60fps); animation should finish far sooner
	done := false
	frames := 0
	for ; frames < maxFrames; frames++ {
		var finished bool
		finished, _ = s.Update()
		// Rendering must never panic at any frame.
		_ = s.View()
		if finished {
			done = true
			break
		}
	}

	if !done {
		t.Fatalf("splash did not complete within %d frames", maxFrames)
	}

	// By completion the reveal must be effectively full.
	if s.revealPos < 0.999 {
		t.Errorf("expected reveal to reach ~1.0, got %f", s.revealPos)
	}

	// A fully revealed frame should contain the tagline text.
	final := s.View()
	if !strings.Contains(final, "Secret Manager") {
		t.Errorf("expected final frame to contain tagline, got:\n%s", final)
	}
}

// TestSplashViewBeforeSize ensures rendering is safe before a window size arrives.
func TestSplashViewBeforeSize(t *testing.T) {
	s := newSplashModel()
	if got := s.View(); got == "" {
		t.Error("expected non-empty view even before size is known")
	}
}
