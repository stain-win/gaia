package tui

import (
	"math"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/harmonica"
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
	splashFPS          = 60                      // Spring simulation / frame rate
	splashTickInterval = time.Second / splashFPS // ~16ms per frame
	splashHoldDuration = 700 * time.Millisecond  // Hold after the logo settles
	dropStartOffset    = 9.0                     // Rows the logo falls from before settling
)

// Spring tuning. angularFrequency controls speed; dampingRatio < 1 overshoots
// (bouncy), == 1 is a smooth critically-damped settle.
const (
	revealFrequency  = 8.0  // Left-to-right reveal speed
	revealDamping    = 1.0  // Critically damped — monotonic, never un-reveals
	dropFrequency    = 7.0  // Entrance drop speed
	dropDamping      = 0.45 // Under-damped — gives the logo a bounce as it lands
	taglineFrequency = 10.0 // Tagline fade-in speed
	taglineDamping   = 1.0
)

// animationTickMsg drives the splash animation forward.
type animationTickMsg time.Time

// splashModel holds the spring-based animation state for the startup splash.
type splashModel struct {
	logoLines  []string
	totalCols  int
	logoHeight int
	width      int
	height     int

	// reveal: 0 → 1, drives how many columns are visible.
	revealSpring harmonica.Spring
	revealPos    float64
	revealVel    float64

	// drop: starts at dropStartOffset, springs to 0 — vertical entrance.
	dropSpring harmonica.Spring
	dropPos    float64
	dropVel    float64

	// tagline: 0 → 1, drives the tagline fade-in once reveal is nearly done.
	taglineSpring harmonica.Spring
	taglinePos    float64
	taglineVel    float64

	holdElapsed time.Duration
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

	dt := harmonica.FPS(splashFPS)
	return &splashModel{
		logoLines:     lines,
		totalCols:     maxCols,
		logoHeight:    len(lines),
		revealSpring:  harmonica.NewSpring(dt, revealFrequency, revealDamping),
		dropSpring:    harmonica.NewSpring(dt, dropFrequency, dropDamping),
		taglineSpring: harmonica.NewSpring(dt, taglineFrequency, taglineDamping),
		dropPos:       dropStartOffset,
	}
}

// tick schedules the next animation frame.
func (s *splashModel) tick() tea.Cmd {
	return tea.Tick(splashTickInterval, func(t time.Time) tea.Msg {
		return animationTickMsg(t)
	})
}

// Init starts the animation tick.
func (s *splashModel) Init() tea.Cmd {
	return s.tick()
}

// Update advances every spring by one frame.
// Returns true when the animation has settled and the app should transition.
func (s *splashModel) Update() (bool, tea.Cmd) {
	s.revealPos, s.revealVel = s.revealSpring.Update(s.revealPos, s.revealVel, 1.0)
	s.dropPos, s.dropVel = s.dropSpring.Update(s.dropPos, s.dropVel, 0.0)

	// The tagline only starts springing in once the logo is mostly revealed.
	if s.revealPos > 0.85 {
		s.taglinePos, s.taglineVel = s.taglineSpring.Update(s.taglinePos, s.taglineVel, 1.0)
	}

	revealed := s.revealPos >= 0.999
	dropSettled := math.Abs(s.dropPos) < 0.02 && math.Abs(s.dropVel) < 0.02
	if revealed && dropSettled {
		s.holdElapsed += splashTickInterval
		if s.holdElapsed >= splashHoldDuration {
			return true, nil // Animation complete
		}
	}

	return false, s.tick()
}

// View renders the current animation frame.
func (s *splashModel) View() string {
	// Color gradient: earthy green → teal across columns.
	colorStart := [3]float64{16, 185, 129} // #10B981
	colorEnd := [3]float64{20, 184, 166}   // #14B8A6

	revealCol := int(math.Round(s.revealPos * float64(s.totalCols)))
	if revealCol < 0 {
		revealCol = 0
	}
	if revealCol > s.totalCols {
		revealCol = s.totalCols
	}

	var renderedLines []string
	for _, line := range s.logoLines {
		runes := []rune(line)
		var builder strings.Builder
		for i, r := range runes {
			if i >= revealCol {
				builder.WriteRune(' ') // Not yet revealed
				continue
			}
			// Gradient position across the full width.
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

	block := strings.Join(renderedLines, "\n")

	// Tagline fades in (dim → light grey) once the logo is fully revealed.
	if revealCol >= s.totalCols && s.taglinePos > 0.01 {
		tg := clamp01(s.taglinePos)
		const dim, lit = 0x33, 0x9c // grey ramp endpoints
		v := dim + int(math.Round(tg*float64(lit-dim)))
		taglineStyled := lipgloss.NewStyle().
			Foreground(lipgloss.Color(sprintf("#%02x%02x%02x", v, v, v))).
			Italic(true).
			Render(splashTagline)
		block += "\n\n" + taglineStyled
	}

	// Before the first WindowSizeMsg we have no viewport; center via Place.
	if s.width <= 0 || s.height <= 0 {
		return block
	}

	// Vertical placement: center the block, then offset by the (springing) drop
	// so the logo falls into place and bounces slightly as it lands.
	contentHeight := lipgloss.Height(block)
	drop := int(math.Round(s.dropPos))
	topPad := (s.height-contentHeight)/2 + drop
	if topPad < 0 {
		topPad = 0
	}

	centered := lipgloss.PlaceHorizontal(s.width, lipgloss.Center, block)
	return strings.Repeat("\n", topPad) + centered
}

// clamp01 constrains v to the [0, 1] range.
func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
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
