package viz

import "github.com/charmbracelet/lipgloss"

// ColorScheme defines the colors used in visualizations
type ColorScheme struct {
	Primary    lipgloss.Color
	Secondary  lipgloss.Color
	Accent     lipgloss.Color
	Background lipgloss.Color
	Text       lipgloss.Color
	Highlight  lipgloss.Color
	Warning    lipgloss.Color
	Error      lipgloss.Color
}

// DefaultColorScheme returns the default color scheme
func DefaultColorScheme() ColorScheme {
	return ColorScheme{
		Primary:    lipgloss.Color("#00ff00"), // Green
		Secondary:  lipgloss.Color("#0000ff"), // Blue
		Accent:     lipgloss.Color("#ff0000"), // Red
		Background: lipgloss.Color("#000000"), // Black
		Text:       lipgloss.Color("#ffffff"), // White
		Highlight:  lipgloss.Color("#ffff00"), // Yellow
		Warning:    lipgloss.Color("#ffa500"), // Orange
		Error:      lipgloss.Color("#ff00ff"), // Magenta
	}
}

// ColorSchemes contains all available color schemes
var ColorSchemes = map[string]ColorScheme{
	"default": DefaultColorScheme(),
	"monokai": {
		Primary:    lipgloss.Color("#a6e22e"),
		Secondary:  lipgloss.Color("#66d9ef"),
		Accent:     lipgloss.Color("#f92672"),
		Background: lipgloss.Color("#272822"),
		Text:       lipgloss.Color("#f8f8f2"),
		Highlight:  lipgloss.Color("#e6db74"),
		Warning:    lipgloss.Color("#fd971f"),
		Error:      lipgloss.Color("#ae81ff"),
	},
	"solarized": {
		Primary:    lipgloss.Color("#859900"),
		Secondary:  lipgloss.Color("#268bd2"),
		Accent:     lipgloss.Color("#dc322f"),
		Background: lipgloss.Color("#002b36"),
		Text:       lipgloss.Color("#839496"),
		Highlight:  lipgloss.Color("#b58900"),
		Warning:    lipgloss.Color("#cb4b16"),
		Error:      lipgloss.Color("#d33682"),
	},
	"nord": {
		Primary:    lipgloss.Color("#88c0d0"),
		Secondary:  lipgloss.Color("#81a1c1"),
		Accent:     lipgloss.Color("#5e81ac"),
		Background: lipgloss.Color("#2e3440"),
		Text:       lipgloss.Color("#d8dee9"),
		Highlight:  lipgloss.Color("#ebcb8b"),
		Warning:    lipgloss.Color("#d08770"),
		Error:      lipgloss.Color("#bf616a"),
	},
	"dracula": {
		Primary:    lipgloss.Color("#50fa7b"),
		Secondary:  lipgloss.Color("#8be9fd"),
		Accent:     lipgloss.Color("#ff79c6"),
		Background: lipgloss.Color("#282a36"),
		Text:       lipgloss.Color("#f8f8f2"),
		Highlight:  lipgloss.Color("#f1fa8c"),
		Warning:    lipgloss.Color("#ffb86c"),
		Error:      lipgloss.Color("#ff5555"),
	},
}
