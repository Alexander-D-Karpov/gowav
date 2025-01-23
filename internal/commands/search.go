package commands

import (
	"fmt"
)

func (c *Commander) handleSearch(query string) (string, error) {
	results, err := c.apiClient.SearchSong(query)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if len(results) == 0 {
		return "No results found.", nil
	}

	c.searchResults = make([]SearchResult, len(results))
	for i, song := range results {
		artist := "Unknown"
		if len(song.Authors) > 0 {
			artist = song.Authors[0].Name
		}
		c.searchResults[i] = SearchResult{
			Title:    song.Name,
			Artist:   artist,
			Album:    song.Album.Name,
			Duration: song.Length,
			URL:      song.File,
		}
	}

	return c.formatSearchResults(), nil
}

func (c *Commander) formatSearchResults() string {
	var sb string
	sb += fmt.Sprintf("Found %d results:\n\n", len(c.searchResults))

	for i, r := range c.searchResults {
		sb += fmt.Sprintf("%d. %s\n", i+1, r.Title)
		sb += fmt.Sprintf("   Artist: %s\n", r.Artist)
		sb += fmt.Sprintf("   Album: %s\n", r.Album)
		min := r.Duration / 60
		sec := r.Duration % 60
		sb += fmt.Sprintf("   Duration: %d:%02d\n", min, sec)
		sb += fmt.Sprintf("   URL: %s\n\n", r.URL)
	}
	return sb
}
