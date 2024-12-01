package commands

func (c *Commander) GetSearchResults() []SearchResult {
	return c.searchResults
}

func (c *Commander) ClearSearchResults() {
	c.searchResults = nil
}
