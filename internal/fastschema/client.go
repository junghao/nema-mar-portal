package fastschema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"time"
)

// Client communicates with the FastSchema sidecar.
type Client struct {
	baseURL    string
	httpClient *http.Client
	token      string
}

// NewClient creates a FastSchema client pointing at the given base URL.
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Login authenticates with FastSchema and stores the JWT token.
func (c *Client) Login(username, password string) error {
	payload := LoginRequest{Login: username, Password: password}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal login: %w", err)
	}

	resp, err := c.httpClient.Post(c.baseURL+"/api/auth/local/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login failed (%d): %s", resp.StatusCode, string(b))
	}

	var loginResp LoginResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		return fmt.Errorf("decode login response: %w", err)
	}

	c.token = loginResp.Data.Token
	return nil
}

// ListEATs returns EATs with event_date since the given time, sorted by event_date descending.
func (c *Client) ListEATs(since time.Time) ([]EAT, error) {
	filter := fmt.Sprintf(`{"event_date":{"$gte":"%s"}}`, since.UTC().Format(time.RFC3339))
	u := fmt.Sprintf("%s/api/content/eat?filter=%s&sort=-event_date&limit=100",
		c.baseURL, url.QueryEscape(filter))

	body, err := c.doGet(u)
	if err != nil {
		return nil, err
	}

	var resp ListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode list response: %w", err)
	}

	return resp.Data.Items, nil
}

// GetEAT retrieves a single EAT by ID.
func (c *Client) GetEAT(id int) (*EAT, error) {
	u := fmt.Sprintf("%s/api/content/eat/%d", c.baseURL, id)
	body, err := c.doGet(u)
	if err != nil {
		return nil, err
	}

	var resp SingleResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode single response: %w", err)
	}

	return &resp.Data, nil
}

// GetLatestVersion returns the latest version EAT for the given event title.
func (c *Client) GetLatestVersion(eventTitle string) (*EAT, error) {
	filter := fmt.Sprintf(`{"event_title":{"$eq":"%s"}}`, eventTitle)
	u := fmt.Sprintf("%s/api/content/eat?filter=%s&sort=-version&limit=1",
		c.baseURL, url.QueryEscape(filter))

	body, err := c.doGet(u)
	if err != nil {
		return nil, err
	}

	var resp ListResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode list response: %w", err)
	}

	if len(resp.Data.Items) == 0 {
		return nil, nil
	}

	return &resp.Data.Items[0], nil
}

// ListDistinctEvents returns distinct event titles from the last N days.
func (c *Client) ListDistinctEvents(days int) ([]string, error) {
	since := time.Now().UTC().AddDate(0, 0, -days)
	eats, err := c.ListEATs(since)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var titles []string
	for _, eat := range eats {
		if !seen[eat.EventTitle] {
			seen[eat.EventTitle] = true
			titles = append(titles, eat.EventTitle)
		}
	}

	return titles, nil
}

// CreateEAT creates a new EAT record in FastSchema.
func (c *Client) CreateEAT(eat *EAT) (*EAT, error) {
	payload, err := json.Marshal(eat)
	if err != nil {
		return nil, fmt.Errorf("marshal eat: %w", err)
	}

	body, err := c.doPost(c.baseURL+"/api/content/eat", "application/json", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	var resp SingleResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode create response: %w", err)
	}

	return &resp.Data, nil
}

// UploadFile uploads a file to FastSchema's file manager.
func (c *Client) UploadFile(filename string, data io.Reader) (*File, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, fmt.Errorf("create form file: %w", err)
	}

	if _, err := io.Copy(part, data); err != nil {
		return nil, fmt.Errorf("copy file data: %w", err)
	}

	if err := writer.Close(); err != nil {
		return nil, fmt.Errorf("close multipart writer: %w", err)
	}

	body, err := c.doPost(c.baseURL+"/api/file/upload", writer.FormDataContentType(), &buf)
	if err != nil {
		return nil, err
	}

	var resp FileResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode file response: %w", err)
	}

	return &resp.Data, nil
}

// ApplySchema sends the schema JSON to FastSchema's admin API.
func (c *Client) ApplySchema(schemaJSON []byte) error {
	_, err := c.doPost(c.baseURL+"/api/schema", "application/json", bytes.NewReader(schemaJSON))
	return err
}

// doGet performs an authenticated GET request and returns the response body.
func (c *Client) doGet(url string) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("FastSchema error (%d): %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// doPost performs an authenticated POST request and returns the response body.
func (c *Client) doPost(url, contentType string, body io.Reader) ([]byte, error) {
	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", contentType)
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("FastSchema error (%d): %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}
