package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

type Song struct {
	Name         string   `json:"name"`
	Slug         string   `json:"slug"`
	File         string   `json:"file"`
	ImageCropped string   `json:"image_cropped"`
	Length       int      `json:"length"`
	Album        Album    `json:"album"`
	Authors      []Author `json:"authors"`
}

type Album struct {
	Name         string   `json:"name"`
	Slug         string   `json:"slug"`
	ImageCropped string   `json:"image_cropped"`
	Authors      []Author `json:"authors"`
}

type Author struct {
	Name         string `json:"name"`
	Slug         string `json:"slug"`
	ImageCropped string `json:"image_cropped"`
}

type SearchResponse struct {
	Count    int    `json:"count"`
	Next     string `json:"next"`
	Previous string `json:"previous"`
	Results  []Song `json:"results"`
}

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient() *Client {
	return &Client{
		baseURL:    "https://new.akarpov.ru/api/v1",
		httpClient: &http.Client{},
	}
}

func (c *Client) SearchSong(query string) ([]Song, error) {
	endpoint := fmt.Sprintf("%s/music/song/?search=%s", c.baseURL, url.QueryEscape(query))

	resp, err := c.httpClient.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var searchResp SearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&searchResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return searchResp.Results, nil
}

func formatSearchResults(songs []Song) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d results:\n\n", len(songs)))

	for i, song := range songs {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, song.Name))
		if len(song.Authors) > 0 {
			sb.WriteString(fmt.Sprintf("   Artist: %s\n", song.Authors[0].Name))
		} else {
			sb.WriteString("   Artist: Unknown\n")
		}
		sb.WriteString(fmt.Sprintf("   Album: %s\n", song.Album.Name))
		sb.WriteString(fmt.Sprintf("   Duration: %d seconds\n", song.Length))
		sb.WriteString(fmt.Sprintf("   URL: %s\n\n", song.File))
	}

	return sb.String()
}
