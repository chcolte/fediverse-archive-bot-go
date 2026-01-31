package mastodon

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
)

// AppCredentials holds the registered app credentials
type AppCredentials struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Name         string `json:"name"`
}

// TokenResponse holds the OAuth token response
type TokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"`
	CreatedAt   int64  `json:"created_at"`
}

// CachedToken holds token info with metadata
type CachedToken struct {
	AccessToken  string
	ClientID     string
	ClientSecret string
	CreatedAt    time.Time
}

// TokenCache provides thread-safe caching for tokens
type TokenCache struct {
	mu      sync.RWMutex
	entries map[string]*CachedToken
}

var (
	httpClientAuth = &http.Client{
		Timeout: 15 * time.Second,
	}
	// DefaultTokenCache is the global token cache
	DefaultTokenCache = NewTokenCache()
)

const (
	appName      = "FediverseArchiveBot"
	redirectURI  = "urn:ietf:wg:oauth:2.0:oob"
	defaultScope = "read:statuses"
)

// NewTokenCache creates a new TokenCache
func NewTokenCache() *TokenCache {
	return &TokenCache{
		entries: make(map[string]*CachedToken),
	}
}

// Get retrieves a cached token if available
func (c *TokenCache) Get(host string) *CachedToken {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.entries[host]
}

// Set stores a token in the cache
func (c *TokenCache) Set(host string, token *CachedToken) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.entries[host] = token
}

// GetAccessToken retrieves or creates an access token for the given host
func GetAccessToken(host string) (string, error) {
	return GetAccessTokenWithCache(host, DefaultTokenCache)
}

// GetAccessTokenWithCache retrieves or creates an access token using a specific cache
func GetAccessTokenWithCache(host string, cache *TokenCache) (string, error) {
	// Check cache first
	if cached := cache.Get(host); cached != nil {
		logger.Debugf("Token cache hit for %s", host)
		return cached.AccessToken, nil
	}

	logger.Debugf("Token cache miss for %s, registering app...", host)

	// Step 1: Register app
	creds, err := registerApp(host)
	if err != nil {
		return "", fmt.Errorf("failed to register app: %w", err)
	}

	logger.Debugf("App registered for %s, getting token...", host)

	// Step 2: Get access token
	token, err := getClientCredentialsToken(host, creds.ClientID, creds.ClientSecret)
	if err != nil {
		return "", fmt.Errorf("failed to get token: %w", err)
	}

	// Store in cache
	cache.Set(host, &CachedToken{
		AccessToken:  token.AccessToken,
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		CreatedAt:    time.Now(),
	})

	logger.Infof("Successfully obtained token for %s", host)
	return token.AccessToken, nil
}

// registerApp registers a new OAuth app on the Mastodon server
func registerApp(host string) (*AppCredentials, error) {
	apiURL := fmt.Sprintf("https://%s/api/v1/apps", host)

	data := url.Values{}
	data.Set("client_name", appName)
	data.Set("redirect_uris", redirectURI)
	data.Set("scopes", defaultScope)

	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "FediverseArchiveBot/1.0")

	resp, err := httpClientAuth.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("app registration failed with status %d", resp.StatusCode)
	}

	var creds AppCredentials
	if err := json.NewDecoder(resp.Body).Decode(&creds); err != nil {
		return nil, fmt.Errorf("failed to decode app credentials: %w", err)
	}

	return &creds, nil
}

// getClientCredentialsToken obtains an access token using client credentials grant
func getClientCredentialsToken(host, clientID, clientSecret string) (*TokenResponse, error) {
	tokenURL := fmt.Sprintf("https://%s/oauth/token", host)

	data := url.Values{}
	data.Set("client_id", clientID)
	data.Set("client_secret", clientSecret)
	data.Set("grant_type", "client_credentials")
	data.Set("scope", defaultScope)

	req, err := http.NewRequest("POST", tokenURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "FediverseArchiveBot/1.0")

	resp, err := httpClientAuth.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token request failed with status %d", resp.StatusCode)
	}

	var token TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&token); err != nil {
		return nil, fmt.Errorf("failed to decode token response: %w", err)
	}

	return &token, nil
}
