package tinify

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	APIURL = "https://api.tinify.com/shrink"
)

type Client struct {
	APIKey string
	Client *http.Client
}

type Options struct {
	// Future proofing for resize etc if needed, though strictly compression requested.
}

type APIError struct {
	StatusCode int
	Type       string `json:"error"`
	Message    string `json:"message"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("api error %d (%s): %s", e.StatusCode, e.Type, e.Message)
}

func NewClient(apiKey string) *Client {
	return &Client{
		APIKey: apiKey,
		Client: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// ValidateKey performs a lightweight check.
// TinyPNG doesn't have a dedicated "verify" endpoint, but we can try to compress a tiny dummy buffer.
// Actually, sending no data might trigger auth check before body check, or we can just try to shrink a 1x1 png.
func (c *Client) ValidateKey(ctx context.Context) error {
	// Minimal 1x1 transparent PNG
	minimalPNG := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d,
		0x49, 0x48, 0x44, 0x52, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x01, 0x00, 0x00, 0x00, 0x00, 0x37, 0x6e, 0xf9, 0x24, 0x00, 0x00, 0x00,
		0x0a, 0x49, 0x44, 0x41, 0x54, 0x08, 0x99, 0x63, 0x60, 0x00, 0x00, 0x00,
		0x02, 0x00, 0x01, 0x73, 0x75, 0x01, 0x18, 0x00, 0x00, 0x00, 0x00, 0x49,
		0x45, 0x4e, 0x44, 0xae, 0x42, 0x60, 0x82,
	}

	_, _, _, err := c.Compress(ctx, bytes.NewReader(minimalPNG), "test.png")
	return err
}

type shrinkResponse struct {
	Input struct {
		Size int64  `json:"size"`
		Type string `json:"type"`
	} `json:"input"`
	Output struct {
		Size  int64  `json:"size"`
		Type  string `json:"type"`
		Width int    `json:"width"`
		Ratio float64 `json:"ratio"`
		URL   string `json:"url"`
	} `json:"output"`
	Error   string `json:"error"`
	Message string `json:"message"`
}

// Compress returns the compressed data reader, the output size, the original size, and error.
// It handles retries internally for 5xx errors or network glitches, but logic calls for "2 retries + backoff".
func (c *Client) Compress(ctx context.Context, r io.Reader, filename string) (io.ReadCloser, int64, int64, error) {
	var body bytes.Buffer
	// We read everything into memory? Or stream?
	// net/http Client.Do with a Reader body will stream if it fits.
	// But TinyPNG API requires Content-Length usually or chunked. Go handles chunked.
	// Let's copy to buffer to be safe and retryable.
	// NOTE: "file too large" handling?
	// If file is huge, memory might be an issue. But typically web images are < 20MB.
	// Let's assume buffering is okay for now.
	if _, err := io.Copy(&body, r); err != nil {
		return nil, 0, 0, err
	}
	originalSize := int64(body.Len())
	payload := body.Bytes()

	apiResp, err := c.doShrinkWithRetry(ctx, payload)
	if err != nil {
		return nil, 0, originalSize, err
	}

	// Download result
	dlResp, err := c.downloadWithRetry(ctx, apiResp.Output.URL)
	if err != nil {
		return nil, 0, originalSize, err
	}

	return dlResp, apiResp.Output.Size, originalSize, nil
}

func (c *Client) doShrinkWithRetry(ctx context.Context, payload []byte) (*shrinkResponse, error) {
	maxRetries := 2
	baseDelay := 1 * time.Second

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			msg := fmt.Sprintf("Retrying upload... (%d/%d)", i, maxRetries)
			// We can maybe log this or have a callback, for now just sleep
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(baseDelay * time.Duration(1<<i)):
				// exponential backoff
			}
			// In TUI, we might want to signal "Retrying" via channel or status.
			// Ideally pipeline handles this via error or specific callback.
			fmt.Println(msg)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", APIURL, bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", "Basic "+basicAuth(c.APIKey, ""))
		req.Header.Set("Content-Type", "application/octet-stream")

		resp, err := c.Client.Do(req)
		if err != nil {
			// Network failure, retry
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 401 || resp.StatusCode == 429 {
			// Invalid key or quota exceeded - do NOT retry
			var apiErr shrinkResponse
			_ = json.NewDecoder(resp.Body).Decode(&apiErr)
			return nil, &APIError{StatusCode: resp.StatusCode, Type: apiErr.Error, Message: apiErr.Message}
		}

		if resp.StatusCode >= 500 {
			// Server error, retry
			continue
		}

		if resp.StatusCode >= 400 {
			// Client error (e.g. bad format), do not retry
			var apiErr shrinkResponse
			_ = json.NewDecoder(resp.Body).Decode(&apiErr)
			return nil, &APIError{StatusCode: resp.StatusCode, Type: apiErr.Error, Message: apiErr.Message}
		}

		// Success
		var result shrinkResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			return nil, err
		}
		return &result, nil
	}
	return nil, fmt.Errorf("max retries exceeded for upload")
}

func (c *Client) downloadWithRetry(ctx context.Context, url string) (io.ReadCloser, error) {
	maxRetries := 2
	baseDelay := 1 * time.Second

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(baseDelay * time.Duration(1<<i)):
			}
		}

		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.Client.Do(req)
		if err != nil {
			continue
		}

		if resp.StatusCode != 200 {
			resp.Body.Close()
			if resp.StatusCode >= 500 {
				continue
			}
			return nil, fmt.Errorf("download failed: %s", resp.Status)
		}

		return resp.Body, nil
	}
	return nil, fmt.Errorf("max retries exceeded for download")
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64Encode([]byte(auth))
}

func base64Encode(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}
