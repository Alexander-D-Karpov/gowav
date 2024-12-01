package commands

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/term"
	"strings"
)

func (c *Commander) handleSearch(query string) (string, error, tea.Cmd) {
	results, err := c.apiClient.SearchSong(query)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err), nil
	}

	if len(results) == 0 {
		return "No results found.", nil, nil
	}

	c.searchResults = make([]SearchResult, len(results))
	for i, song := range results {
		artists := "Unknown Artist"
		if len(song.Authors) > 0 {
			artists = song.Authors[0].Name
		}

		c.searchResults[i] = SearchResult{
			Title:    song.Name,
			Artist:   artists,
			Album:    song.Album.Name,
			Duration: song.Length,
			URL:      song.File,
		}
	}

	return c.formatSearchResults(), nil, nil
}

func (c *Commander) formatSearchResults() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d results (use number to load):\n\n", len(c.searchResults)))

	width := getTerminalWidth()
	divider := strings.Repeat("─", width)

	for i, result := range c.searchResults {
		sb.WriteString(divider + "\n")
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, result.Title))

		// Create two columns
		details := fmt.Sprintf("Artist: %-20s │ Album: %-25s │ Duration: %d:%02d",
			truncateString(result.Artist, 20),
			truncateString(result.Album, 25),
			result.Duration/60,
			result.Duration%60)

		sb.WriteString(details + "\n")
	}
	sb.WriteString(divider + "\n")

	return sb.String()
}

func truncateString(s string, length int) string {
	if len(s) <= length {
		return s
	}
	return s[:length-3] + "..."
}

func getTerminalWidth() int {
	if width, _, err := term.GetSize(0); err == nil && width > 0 {
		return width
	}
	return 80 // default width
}
