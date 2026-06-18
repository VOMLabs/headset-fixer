package openrouter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	Model  = "openrouter/owl-alpha"
	APIURL = "https://openrouter.ai/api/v1/chat/completions"
)

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type request struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type choice struct {
	Message message `json:"message"`
}

type response struct {
	Choices []choice `json:"choices"`
}

func SendText(text, apiKey string) (string, error) {
	body, err := json.Marshal(request{
		Model: Model,
		Messages: []message{
			{
				Role:    "system",
				Content: "You convert plain text into clean Markdown. Preserve meaning, structure it well with headings, lists, and code blocks. No commentary.",
			},
			{
				Role:    "user",
				Content: text,
			},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", APIURL, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request: %w", err)
	}
	defer resp.Body.Close()

	respData, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respData))
	}

	var apiResp response
	if err := json.Unmarshal(respData, &apiResp); err != nil {
		return "", fmt.Errorf("parse response: %w", err)
	}

	if len(apiResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	return apiResp.Choices[0].Message.Content, nil
}
