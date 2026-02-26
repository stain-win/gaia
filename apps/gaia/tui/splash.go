package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Large ASCII logo using half-block characters for organic/chubby aesthetic.
// Inspired by Gaia — Greek goddess of Earth.
const gaiaLogoLarge = `                                                  .#@-                                          
                                                   .-=.                                          
                   .-*+:                 .-=-.                           .:=-.                   
            .-=++=:-@@@@*      .-*###*-.*@@@@@    .+#%*:       .:+###*=.-@@@@@.                  
         .-@@@@@@=-%@@@@#    .*@@@@@*.:%@@@@@@   .#@@@@#     .-@@@@@#:.*@@@@@@:                  
        :%@@@@@*. .@@@@@#   :@@@@@@-  .*@@@@@@   :@@@@@%.   .#@@@@@+.  -@@@@@@.                  
       .@@@@@@*. .+@@@@@*  .#@@@@@=   .#@@@@@%   =@@@@@%   .+@@@@@*   .+@@@@@@.                  
      .#@@@@@@.  :@@@@@@=  -@@@@@%.   -@@@@@@-   =@@@@@#   :@@@@@@-   :%@@@@@*   .+.             
      .@@@@@@+  .*@@@@@@. .+@@@@@*.   +@@@@@@.   =@@@@@+   :@@@@@%.   -@@@@@@=   :*              
      .%@@@@@:  -@@@@@@%.:%*@@@@@+.  .@@@@@@@.  .@@@@@@=  .@@@@@@#.  .%@@@@@-   *-              
      :@@@@@=.=%+@@@@@+**..#@@@@+. .@=*@@@@%. .@-#@@@@- .#:+@@@@#. .#*=@@@@@: .#+.              
        .=@@@@%-.+@@@@@=.   .#@@@@::%- .@@@@%.:%. .@@@@-=*. .+@@@@+.#+..#@@@@-.*-.               
                .#@@@@+.      .=*##+.   .*@@@@+.   .-**+.     .-+##+:.   =%@@@#:.                
              .+@@@@@%.                   ....                            ....                   
           .:#*:+@@@%.                                                                           
           :#  :@@@#.                                                                            
           :#=+@@@=.                                                                             
             .=-.                                                                                
                                                                                                 
                                                                                                 `

// Compact logo: mini vault glyph with G for headers after splash.
const gaiaCompactGlyph = `╭─────╮
│█████│
│█ G █│
╰─────╯`

// splashTagline shown below the large logo during splash.
const splashTagline = "✦ Secret Manager ✦"

// Animation timing
const (
	splashTickInterval = 55 * time.Millisecond  // Speed of column reveal
	splashHoldDuration = 800 * time.Millisecond // Hold after full reveal
)

// Animation phases
const (
	phaseReveal     = iota // Letters appearing left-to-right
	phaseHold              // Full logo on display
	phaseTransition        // Shrink to compact
)

// animationTickMsg drives the splash animation forward.
type animationTickMsg time.Time

// splashModel holds the animation state for the startup splash.
type splashModel struct {
	phase       int
	revealCol   int // Current column being revealed (left-to-right)
	totalCols   int // Total columns in the logo
	holdElapsed time.Duration
	logoLines   []string
	width       int
	height      int
}

// newSplashModel creates a fresh splash animation model.
func newSplashModel() *splashModel {
	lines := strings.Split(gaiaLogoLarge, "\n")
	maxCols := 0
	for _, line := range lines {
		runeCount := len([]rune(line))
		if runeCount > maxCols {
			maxCols = runeCount
		}
	}

	return &splashModel{
		phase:     phaseReveal,
		revealCol: 0,
		totalCols: maxCols,
		logoLines: lines,
	}
}

// Init starts the animation tick.
func (s *splashModel) Init() tea.Cmd {
	return tea.Tick(splashTickInterval, func(t time.Time) tea.Msg {
		return animationTickMsg(t)
	})
}

// Update advances the splash animation by one tick.
// Returns true when the animation is complete and the app should transition.
func (s *splashModel) Update() (bool, tea.Cmd) {
	switch s.phase {
	case phaseReveal:
		s.revealCol += 2 // Reveal 2 columns per tick for smooth speed
		if s.revealCol >= s.totalCols {
			s.revealCol = s.totalCols
			s.phase = phaseHold
		}
		return false, tea.Tick(splashTickInterval, func(t time.Time) tea.Msg {
			return animationTickMsg(t)
		})

	case phaseHold:
		s.holdElapsed += splashTickInterval
		if s.holdElapsed >= splashHoldDuration {
			s.phase = phaseTransition
			return true, nil // Animation complete
		}
		return false, tea.Tick(splashTickInterval, func(t time.Time) tea.Msg {
			return animationTickMsg(t)
		})

	case phaseTransition:
		return true, nil
	}

	return false, nil
}

// View renders the current animation frame.
func (s *splashModel) View() string {
	// Color gradient: earthy green → teal across columns
	colorStart := [3]float64{16, 185, 129} // #10B981
	colorEnd := [3]float64{20, 184, 166}   // #14B8A6

	var renderedLines []string
	for _, line := range s.logoLines {
		runes := []rune(line)
		var builder strings.Builder
		for i, r := range runes {
			if i >= s.revealCol {
				builder.WriteRune(' ') // Not yet revealed
				continue
			}
			// Calculate gradient position
			t := 0.0
			if s.totalCols > 1 {
				t = float64(i) / float64(s.totalCols-1)
			}
			cr := colorStart[0] + t*(colorEnd[0]-colorStart[0])
			cg := colorStart[1] + t*(colorEnd[1]-colorStart[1])
			cb := colorStart[2] + t*(colorEnd[2]-colorStart[2])

			color := lipgloss.Color(sprintf("#%02x%02x%02x", int(cr), int(cg), int(cb)))
			styled := lipgloss.NewStyle().
				Foreground(color).
				Bold(true).
				Render(string(r))
			builder.WriteString(styled)
		}
		renderedLines = append(renderedLines, builder.String())
	}

	logo := strings.Join(renderedLines, "\n")

	// Add tagline and version below logo (only when fully revealed)
	var below string
	if s.revealCol >= s.totalCols {
		taglineStyled := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9CA3AF")).
			Italic(true).
			Render(splashTagline)

		below = "\n\n" + taglineStyled
	}

	content := logo + below

	// Center in viewport
	return lipgloss.Place(
		s.width,
		s.height,
		lipgloss.Center,
		lipgloss.Center,
		content,
	)
}

// renderCompactHeader renders the compact vault header with version.
// Left: mini vault, name and version aligned on lower rows, bottom border under header.
func renderCompactHeader(width int) string {
	// Mini vault glyph (4 lines tall)
	globeLines := strings.Split(gaiaCompactGlyph, "\n")

	globeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#10B981")).
		Bold(true)

	nameStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF8C00")).
		Bold(true)

	versionStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#9CA3AF"))

	// Build the left side: name and version aligned to lower rows.
	var leftLines [4]string
	for i, gl := range globeLines {
		switch i {
		case 2:
			leftLines[i] = globeStyle.Render(gl) + "  " + nameStyle.Render("gaia")
		case 3:
			leftLines[i] = globeStyle.Render(gl) + "  " + versionStyle.Render("v"+getVersion())
		default:
			leftLines[i] = globeStyle.Render(gl)
		}
	}

	// Compose each line with spacing to fill the row width.
	var composedLines []string
	for i := 0; i < 4; i++ {
		leftWidth := lipgloss.Width(leftLines[i])
		padding := width - leftWidth - 2
		if padding < 0 {
			padding = 0
		}
		composedLines = append(composedLines, leftLines[i]+strings.Repeat(" ", padding))
	}

	// Add one padding row and a bottom border.
	innerWidth := width - 2
	if innerWidth < 0 {
		innerWidth = 0
	}
	composedLines = append(composedLines, strings.Repeat("─", innerWidth))
	composedLines = append(composedLines, strings.Repeat(" ", innerWidth))

	return lipgloss.NewStyle().
		Padding(0, 1).
		Render(strings.Join(composedLines, "\n"))
}

// sprintf is a local helper to avoid importing fmt just for color formatting.
func sprintf(format string, a ...interface{}) string {
	// Simple hex color formatter
	if format == "#%02x%02x%02x" && len(a) == 3 {
		r, g, b := a[0].(int), a[1].(int), a[2].(int)
		const hex = "0123456789abcdef"
		return "#" + string([]byte{hex[r/16], hex[r%16], hex[g/16], hex[g%16], hex[b/16], hex[b%16]})
	}
	return ""
}

// getVersion returns the app version. Falls back to "dev" if not set.
func getVersion() string {
	// This will be populated from the cmd package's linker flags.
	// For now, return a sensible default.
	return "dev"
}
