package viz

import (
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"time"
)

// Time utilities
func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	d = d.Round(time.Second)
	min := int(d.Minutes())
	sec := int(d.Seconds()) % 60
	return fmt.Sprintf("%02d:%02d", min, sec)
}

// Sample conversion helpers
func samplesToTime(samples int, sampleRate int) time.Duration {
	seconds := float64(samples) / float64(sampleRate)
	return time.Duration(seconds * float64(time.Second))
}

func timeToSamples(t time.Duration, sampleRate int) int {
	seconds := t.Seconds()
	return int(seconds * float64(sampleRate))
}

// Drawing helpers
func createBar(width int, fill float64, style lipgloss.Style) string {
	if width < 1 {
		return ""
	}

	filled := int(float64(width) * fill)
	if filled > width {
		filled = width
	}

	bar := strings.Repeat("█", filled)
	if filled < width {
		bar += strings.Repeat("░", width-filled)
	}

	return style.Render(bar)
}

// Color helpers
func getGradientColor(intensity float64, scheme ColorScheme) lipgloss.Color {
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}
	r1, g1, b1 := hexToRGB(string(scheme.Primary))
	r2, g2, b2 := hexToRGB(string(scheme.Secondary))

	r := int(float64(r1) + intensity*float64(r2-r1))
	g := int(float64(g1) + intensity*float64(g2-g1))
	b := int(float64(b1) + intensity*float64(b2-b1))

	return lipgloss.Color(fmt.Sprintf("#%02x%02x%02x", r, g, b))
}

func hexToRGB(hex string) (int, int, int) {
	hex = strings.TrimPrefix(hex, "#")
	if len(hex) == 3 {
		hex = string(hex[0]) + string(hex[0]) +
			string(hex[1]) + string(hex[1]) +
			string(hex[2]) + string(hex[2])
	}

	var r, g, b int
	fmt.Sscanf(hex, "%02x%02x%02x", &r, &g, &b)
	return r, g, b
}

// Math helpers
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func scaleValue(value, min, max, targetMin, targetMax float64) float64 {
	if max == min {
		return targetMin
	}
	scaled := (value - min) / (max - min)
	return targetMin + scaled*(targetMax-targetMin)
}
