package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	token string
	http  *http.Client
	base  string
}

func New(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 65 * time.Second},
		base:  "https://api.telegram.org/bot" + token,
	}
}

type Update struct {
	UpdateID int `json:"update_id"`
	Message  *struct {
		MessageID int `json:"message_id"`
		From      struct {
			FirstName string `json:"first_name"`
			Username  string `json:"username"`
		} `json:"from"`
		Chat struct {
			ID int64 `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"message"`
}

type getUpdatesResp struct {
	OK     bool     `json:"ok"`
	Result []Update `json:"result"`
}

func (c *Client) GetUpdates(ctx context.Context, offset, timeout int) ([]Update, error) {
	q := url.Values{}
	q.Set("offset", fmt.Sprintf("%d", offset))
	q.Set("timeout", fmt.Sprintf("%d", timeout))
	q.Set("allowed_updates", `["message"]`)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.base+"/getUpdates?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var r getUpdatesResp
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return nil, err
	}
	return r.Result, nil
}

type sendResult struct {
	OK          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
}

func (c *Client) send(ctx context.Context, chatID any, html string) (sendResult, error) {
	payload := map[string]any{
		"chat_id":                  chatID,
		"text":                     html,
		"parse_mode":               "HTML",
		"disable_web_page_preview": true,
	}
	raw, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/sendMessage", bytes.NewReader(raw))
	if err != nil {
		return sendResult{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return sendResult{}, err
	}
	defer resp.Body.Close()
	var r sendResult
	_ = json.NewDecoder(resp.Body).Decode(&r)
	return r, nil
}

func (c *Client) Send(ctx context.Context, chatID int64, html string) (blocked bool, err error) {
	r, err := c.send(ctx, chatID, html)
	if err != nil {
		return false, err
	}
	if r.OK {
		return false, nil
	}
	if r.ErrorCode == 403 || strings.Contains(r.Description, "bot was blocked") || strings.Contains(r.Description, "chat not found") {
		return true, fmt.Errorf("telegram: %s", r.Description)
	}
	return false, fmt.Errorf("telegram: %s", r.Description)
}

func (c *Client) SendChannel(ctx context.Context, channel, html string) error {
	var chat any = channel
	if n, perr := strconv.ParseInt(channel, 10, 64); perr == nil {
		chat = n
	}
	r, err := c.send(ctx, chat, html)
	if err != nil {
		return err
	}
	if !r.OK {
		return fmt.Errorf("telegram: %s", r.Description)
	}
	return nil
}

func (c *Client) Broadcast(ctx context.Context, chatIDs []int64, html string, onBlocked func(int64)) {
	const ratePerSec = 25
	ticker := time.NewTicker(time.Second / ratePerSec)
	defer ticker.Stop()
	sem := make(chan struct{}, 8)
	done := make(chan struct{}, len(chatIDs))
	for _, id := range chatIDs {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		sem <- struct{}{}
		go func(chat int64) {
			defer func() { <-sem; done <- struct{}{} }()
			blocked, _ := c.Send(ctx, chat, html)
			if blocked && onBlocked != nil {
				onBlocked(chat)
			}
		}(id)
	}
	for range chatIDs {
		<-done
	}
}

func EscapeHTML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
