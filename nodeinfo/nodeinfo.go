package nodeinfo

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
)

// WellKnownNodeInfo represents the response from /.well-known/nodeinfo
type WellKnownNodeInfo struct {
	Links []WellKnownLink `json:"links"`
}

type WellKnownLink struct {
	Rel  string `json:"rel"`
	Href string `json:"href"`
}

// NodeInfo represents the actual NodeInfo response
type NodeInfo struct {
	Version  string `json:"version"`
	Software struct {
		Name       string `json:"name"`
		Version    string `json:"version"`
		Repository string `json:"repository,omitempty"` // 2.1+
		Homepage   string `json:"homepage,omitempty"`   // 2.1+
	} `json:"software"`
	Protocols         []string `json:"protocols"`
	OpenRegistrations bool     `json:"openRegistrations"`
	RawJSON           string   `json:"-"` // Original JSON response (not part of the schema)
}

// CacheEntry holds a cached NodeInfo with its fetch time
type CacheEntry struct {
	NodeInfo  *NodeInfo
	FetchedAt time.Time
}

// Cache provides thread-safe caching for NodeInfo
type Cache struct {
	mu      sync.RWMutex
	entries map[string]*CacheEntry
	ttl     time.Duration
}

var (
	httpClient = &http.Client{
		Timeout: 10 * time.Second,
	}
	// DefaultCache is the global cache instance
	DefaultCache = NewCache(24 * time.Hour)
)

// NewCache creates a new Cache with the specified TTL
func NewCache(ttl time.Duration) *Cache {
	return &Cache{
		entries: make(map[string]*CacheEntry),
		ttl:     ttl,
	}
}

// Get retrieves a cached NodeInfo if available and not expired
func (c *Cache) Get(host string) (*NodeInfo, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	entry, exists := c.entries[host]
	if !exists {
		return nil, false
	}

	if time.Since(entry.FetchedAt) > c.ttl {
		return nil, false
	}

	return entry.NodeInfo, true
}

// Set stores a NodeInfo in the cache
func (c *Cache) Set(host string, nodeInfo *NodeInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.entries[host] = &CacheEntry{
		NodeInfo:  nodeInfo,
		FetchedAt: time.Now(),
	}
}

// GetAll returns a map of all cached entries (for saving/debugging)
func (c *Cache) GetAll() map[string]*CacheEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make(map[string]*CacheEntry, len(c.entries))
	for k, v := range c.entries {
		result[k] = v
	}
	return result
}

// Size returns the number of cached entries
func (c *Cache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.entries)
}

// GetSoftwareName fetches the software name for a given host (with caching)
// host should be just the domain (e.g., "mstdn.jp")
func GetSoftwareName(host string) (string, error) {
	nodeInfo, err := GetNodeInfo(host)
	if err != nil {
		return "", err
	}
	return nodeInfo.Software.Name, nil
}

// GetNodeInfo fetches the full NodeInfo for a given host (with caching)
func GetNodeInfo(host string) (*NodeInfo, error) {
	return GetNodeInfoWithCache(host, DefaultCache)
}

// GetNodeInfoWithCache fetches NodeInfo using a specific cache
func GetNodeInfoWithCache(host string, cache *Cache) (*NodeInfo, error) {
	// Check cache first
	if cached, ok := cache.Get(host); ok {
		logger.Debugf("NodeInfo cache hit for %s", host)
		return cached, nil
	}

	logger.Debugf("NodeInfo cache miss for %s, fetching...", host)

	// Fetch from server
	nodeInfo, err := fetchNodeInfo(host)
	if err != nil {
		return nil, err
	}

	// Store in cache
	cache.Set(host, nodeInfo)

	return nodeInfo, nil
}

// fetchNodeInfo performs the actual HTTP requests to get NodeInfo
func fetchNodeInfo(host string) (*NodeInfo, error) {
	// Step 1: Fetch /.well-known/nodeinfo
	wellKnownURL := fmt.Sprintf("https://%s/.well-known/nodeinfo", host)

	resp, err := httpClient.Get(wellKnownURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch well-known nodeinfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("well-known nodeinfo returned status %d", resp.StatusCode)
	}

	var wellKnown WellKnownNodeInfo
	if err := json.NewDecoder(resp.Body).Decode(&wellKnown); err != nil {
		return nil, fmt.Errorf("failed to decode well-known nodeinfo: %w", err)
	}

	// Step 2: Find the best schema version (prefer highest version)
	nodeInfoURL := findBestNodeInfoURL(wellKnown.Links)
	if nodeInfoURL == "" {
		return nil, fmt.Errorf("no supported nodeinfo schema found")
	}

	logger.Debugf("Fetching NodeInfo from: %s", nodeInfoURL)

	// Step 3: Fetch actual NodeInfo
	resp2, err := httpClient.Get(nodeInfoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch nodeinfo: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("nodeinfo returned status %d", resp2.StatusCode)
	}

	// Read raw body for storage
	rawBody, err := io.ReadAll(resp2.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read nodeinfo body: %w", err)
	}

	var nodeInfo NodeInfo
	if err := json.Unmarshal(rawBody, &nodeInfo); err != nil {
		return nil, fmt.Errorf("failed to decode nodeinfo: %w", err)
	}

	// Store the raw JSON
	nodeInfo.RawJSON = string(rawBody)

	return &nodeInfo, nil
}

// findBestNodeInfoURL returns the URL for the highest supported schema version
func findBestNodeInfoURL(links []WellKnownLink) string {
	// Prefer higher versions
	schemaVersions := []string{
		"http://nodeinfo.diaspora.software/ns/schema/2.1",
		"http://nodeinfo.diaspora.software/ns/schema/2.0",
		"http://nodeinfo.diaspora.software/ns/schema/1.1",
		"http://nodeinfo.diaspora.software/ns/schema/1.0",
	}

	for _, schema := range schemaVersions {
		for _, link := range links {
			if strings.EqualFold(link.Rel, schema) {
				return link.Href
			}
		}
	}

	// Fallback: return first link if any
	if len(links) > 0 {
		return links[0].Href
	}

	return ""
}