package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

const (
	clientTimeout = 30 * time.Second
)

// apiClient is a thin wrapper around http.Client that manages base URL and
// authentication for the RemnaCore admin API.
type apiClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// newAPIClient returns a configured API client. The token is resolved from the
// flag value first, then from the VPNCTL_API_TOKEN environment variable.
func newAPIClient() *apiClient {
	token := apiToken
	if token == "" {
		token = os.Getenv(EnvAPIToken)
	}
	return &apiClient{
		baseURL: apiURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: clientTimeout,
		},
	}
}

func (c *apiClient) do(method, path string, body any) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, 0, fmt.Errorf("marshalling request body: %w", err)
		}
		reqBody = bytes.NewReader(data)
	}

	url := c.baseURL + path
	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		return nil, 0, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("performing request to %s: %w", url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("reading response body: %w", err)
	}

	return respBody, resp.StatusCode, nil
}

func (c *apiClient) get(path string) ([]byte, int, error) {
	return c.do(http.MethodGet, path, nil)
}

func (c *apiClient) post(path string, body any) ([]byte, int, error) {
	return c.do(http.MethodPost, path, body)
}

func (c *apiClient) delete(path string) ([]byte, int, error) {
	return c.do(http.MethodDelete, path, nil)
}
