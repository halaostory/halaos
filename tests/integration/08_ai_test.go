package integration

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAIChat(t *testing.T) {
	resp, status, err := apiPost("/ai/chat", map[string]any{
		"message": "How many employees are there?",
	})
	require.NoError(t, err)

	// Skip if AI is not configured (503 or specific error)
	if status == 503 || status == 500 {
		t.Skipf("AI not configured on server (HTTP %d)", status)
		return
	}

	requireSuccess(t, resp, status)

	data := extractData(t, resp)
	var chatResp struct {
		Response string `json:"response"`
		Message  string `json:"message"`
	}
	json.Unmarshal(data, &chatResp)
	answer := chatResp.Response
	if answer == "" {
		answer = chatResp.Message
	}
	assert.NotEmpty(t, answer, "AI should return a response")
	t.Logf("AI response: %s", answer[:min(200, len(answer))])
}
