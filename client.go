package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type client struct {
	baseURL  string
	token    string
	http     *http.Client
	lastBody []byte
}

func newClient(baseURL, token string) *client {
	return &client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// do sends a JSON request and decodes a JSON response into out (if non-nil).
// On non-2xx, it tries to extract {"error": "..."} or {"errors": [...]} from
// the body and returns a formatted error.
func (c *client) do(method, path string, query url.Values, body any, out any) error {
	full := c.baseURL + path
	if len(query) > 0 {
		full += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal body: %w", err)
		}
		reqBody = bytes.NewReader(buf)
	}

	req, err := http.NewRequest(method, full, reqBody)
	if err != nil {
		return err
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return decodeAPIError(resp.StatusCode, respBody)
	}

	c.lastBody = respBody
	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("decode response: %w (body: %s)", err, string(respBody))
		}
	}
	return nil
}

func decodeAPIError(status int, body []byte) error {
	var generic struct {
		Error   string   `json:"error"`
		Errors  []string `json:"errors"`
		Message string   `json:"message"`
	}
	_ = json.Unmarshal(body, &generic)

	switch {
	case generic.Error != "":
		return fmt.Errorf("HTTP %d: %s", status, generic.Error)
	case len(generic.Errors) > 0:
		msg := strings.Join(generic.Errors, "; ")
		if generic.Message != "" {
			return fmt.Errorf("HTTP %d: %s — %s", status, generic.Message, msg)
		}
		return fmt.Errorf("HTTP %d: %s", status, msg)
	default:
		body = bytes.TrimSpace(body)
		if len(body) == 0 {
			return fmt.Errorf("HTTP %d", status)
		}
		return fmt.Errorf("HTTP %d: %s", status, string(body))
	}
}

// ─── response types ────────────────────────────────────────────────────────

type listSummary struct {
	UID                   string `json:"uid"`
	Name                  string `json:"name"`
	Description           string `json:"description"`
	CreatedAt             string `json:"created_at"`
	UpdatedAt             string `json:"updated_at"`
	URL                   string `json:"url"`
	Shareable             bool   `json:"shareable"`
	ShareableURL          string `json:"shareable_url"`
	RSSFeedURL            string `json:"rss_feed_url"`
	ICalFeedURL           string `json:"ical_feed_url"`
	ItemsCount            int    `json:"items_count"`
	CompletedItemsCount   int    `json:"completed_items_count"`
	UncompletedItemsCount int    `json:"uncompleted_items_count"`
	OverdueItemsCount     int    `json:"overdue_items_count"`
	Archived              bool   `json:"archived"`
}

type listsIndexResponse struct {
	Lists []listSummary `json:"lists"`
}

type listStats struct {
	TotalItems       int `json:"total_items"`
	CompletedItems   int `json:"completed_items"`
	UncompletedItems int `json:"uncompleted_items"`
	ArchivedItems    int `json:"archived_items"`
	OverdueItems     int `json:"overdue_items"`
}

type listDetail struct {
	UID          string    `json:"uid"`
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	CreatedAt    string    `json:"created_at"`
	UpdatedAt    string    `json:"updated_at"`
	URL          string    `json:"url"`
	Shareable    bool      `json:"shareable"`
	ShareableURL string    `json:"shareable_url"`
	RSSFeedURL   string    `json:"rss_feed_url"`
	ICalFeedURL  string    `json:"ical_feed_url"`
	Stats        listStats `json:"stats"`
}

type tab struct {
	ID        int    `json:"id"`
	Name      string `json:"name"`
	Slug      string `json:"slug"`
	Position  int    `json:"position"`
	ItemCount *int   `json:"item_count,omitempty"`
}

type listShowResponse struct {
	List  listDetail   `json:"list"`
	Tabs  []tab        `json:"tabs"`
	Items []embedItem  `json:"items"`
}

// embedItem is the slimmed-down item shape returned inside list show / shared
// list — note that it intentionally lacks `uid`, so for mutations callers
// must use itemsIndex (which returns full items).
type embedItem struct {
	Body        string  `json:"body"`
	Complete    bool    `json:"complete"`
	Position    int     `json:"position"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	DueAt       *string `json:"due_at"`
	CompletedAt *string `json:"completed_at"`
	Archived    bool    `json:"archived"`
	TabID       *int    `json:"tab_id"`
	TabSlug     *string `json:"tab_slug"`
}

type item struct {
	UID         string  `json:"uid"`
	Body        string  `json:"body"`
	Complete    bool    `json:"complete"`
	Position    int     `json:"position"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	DueAt       *string `json:"due_at"`
	CompletedAt *string `json:"completed_at"`
	Archived    bool    `json:"archived"`
	ListUID     string  `json:"list_uid"`
	TabID       *int    `json:"tab_id"`
	TabSlug     *string `json:"tab_slug"`
}

type itemsIndexResponse struct {
	Items []item `json:"items"`
}

type itemEnvelope struct {
	Item    item   `json:"item"`
	Message string `json:"message"`
}

type tabsIndexResponse struct {
	Tabs          []tab `json:"tabs"`
	UntabbedCount int   `json:"untabbed_count"`
}

type tabEnvelope struct {
	Tab     tab    `json:"tab"`
	Message string `json:"message"`
}

type dashboardStats struct {
	ListsCount           int `json:"lists_count"`
	ArchivedListsCount   int `json:"archived_lists_count"`
	OverdueItemsCount    int `json:"overdue_items_count"`
	TotalItems           int `json:"total_items"`
	CompletedItems       int `json:"completed_items"`
	CompletionPercentage int `json:"completion_percentage"`
}

type dashboardResponse struct {
	Stats dashboardStats `json:"stats"`
	Lists []listSummary  `json:"lists"`
}

type overdueItem struct {
	Body        string  `json:"body"`
	Complete    bool    `json:"complete"`
	DueAt       *string `json:"due_at"`
	DaysOverdue int     `json:"days_overdue"`
	List        struct {
		UID  string `json:"uid"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"list"`
}

type overdueResponse struct {
	Count int           `json:"count"`
	Items []overdueItem `json:"items"`
}

type dueItem struct {
	ID           string  `json:"id"`
	Body         string  `json:"body"`
	Complete     bool    `json:"complete"`
	DueAt        *string `json:"due_at"`
	DaysUntilDue int     `json:"days_until_due"`
	List         struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"list"`
}

type dueResponse struct {
	Count int       `json:"count"`
	Items []dueItem `json:"items"`
}

type sharedListResponse struct {
	List  listDetail  `json:"list"`
	Items []embedItem `json:"items"`
}
