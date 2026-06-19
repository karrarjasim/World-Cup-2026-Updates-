package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	base  string
	key   string
	model string
	http  *http.Client
}

func New(baseURL, key, model string) *Client {
	return &Client{
		base:  strings.TrimRight(baseURL, "/"),
		key:   key,
		model: model,
		http:  &http.Client{Timeout: 30 * time.Second},
	}
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string    `json:"model"`
	Messages    []message `json:"messages"`
	Temperature float64   `json:"temperature"`
	MaxTokens   int       `json:"max_tokens"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) generate(ctx context.Context, system, user string, maxTok int) (string, error) {
	if c.key == "" {
		return "", fmt.Errorf("llm: no API key configured")
	}
	msgs := make([]message, 0, 2)
	if system != "" {
		msgs = append(msgs, message{Role: "system", Content: system})
	}
	msgs = append(msgs, message{Role: "user", Content: user})

	raw, _ := json.Marshal(chatRequest{
		Model:       c.model,
		Messages:    msgs,
		Temperature: 0.7,
		MaxTokens:   maxTok,
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/chat/completions", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.key)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var r chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&r); err != nil {
		return "", fmt.Errorf("llm: decode (status %d): %w", resp.StatusCode, err)
	}
	if resp.StatusCode != http.StatusOK {
		if r.Error != nil {
			return "", fmt.Errorf("llm: status %d: %s", resp.StatusCode, r.Error.Message)
		}
		return "", fmt.Errorf("llm: status %d", resp.StatusCode)
	}
	if len(r.Choices) == 0 {
		return "", fmt.Errorf("llm: empty response")
	}
	return strings.TrimSpace(r.Choices[0].Message.Content), nil
}

const analystSystem = `أنت محلل كروي عربي خبير ومتابع لكأس العالم 2026. مهمتك كتابة تعليق تحليلي قصير ودقيق باللغة العربية الفصحى المبسطة عن نتيجة مباراة.
القواعد:
- استخدم بيانات ترتيب المجموعة المعطاة فقط، ولا تخترع أرقاماً أو نتائج.
- اذكر تأثير النتيجة على ترتيب المجموعة وفرص التأهل والسيناريوهات القادمة بإيجاز.
- اكتب أسماء المنتخبات بالعربية.
- الطول: من جملتين إلى أربع جمل فقط. بلا مقدمات ولا عناوين.`

func (c *Client) AnalyzeResult(ctx context.Context, matchLine, groupTable string) (string, error) {
	prompt := fmt.Sprintf("النتيجة النهائية:\n%s\n\nترتيب المجموعة الحالي:\n%s\n\nاكتب التحليل.", matchLine, groupTable)
	return c.generate(ctx, analystSystem, prompt, 400)
}

const assistantSystem = `أنت مساعد ذكي ودود متخصص في كأس العالم 2026، تتحدث العربية فقط.
أجب بإيجاز ووضوح اعتماداً على معطيات السياق المرفقة (المباريات والترتيب).
إذا لم تتوفر معلومة في السياق فاعتذر بلطف وقل إنك ستوافيه بها فور توفرها، ولا تخترع نتائج أو مواعيد.`

func (c *Client) Answer(ctx context.Context, question, contextBlock string) (string, error) {
	prompt := fmt.Sprintf("سياق محدّث:\n%s\n\nسؤال المستخدم: %s", contextBlock, question)
	return c.generate(ctx, assistantSystem, prompt, 600)
}

func (c *Client) Summarize(ctx context.Context, title, snippet string) (string, error) {
	sys := "لخّص الخبر الرياضي التالي في جملة عربية واحدة جذابة ودقيقة بلا مقدمات."
	return c.generate(ctx, sys, title+"\n"+snippet, 120)
}
