package ai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"github.com/Ady0333/loadr/engine"
)

const (
	groqEndpoint = "https://api.groq.com/openai/v1/chat/completions"
	groqModel    = "llama-3.3-70b-versatile"
)

// MetricsSnapshot is re-exported from the engine package so callers can use the
// signature AnalyzeMetrics([]MetricsSnapshot) directly.
type MetricsSnapshot = engine.MetricsSnapshot

// --- Groq / OpenAI-compatible chat completions wire types ---

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []chatMessage   `json:"messages"`
	Temperature    float64         `json:"temperature"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// AnalyzeMetrics sends the collected load-test snapshots to Groq and returns a
// plain-text diagnosis: what's breaking, at what load, and three actionable
// fixes. It returns an error if the metrics can't be encoded or the API call
// fails.
func AnalyzeMetrics(snapshots []MetricsSnapshot) (string, error) {
	if len(snapshots) == 0 {
		return "", fmt.Errorf("no metrics were collected, nothing to analyze")
	}

	data, err := json.MarshalIndent(snapshots, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode metrics: %w", err)
	}

	system := "You are a senior performance engineer analyzing HTTP load-test " +
		"results. Be concise, concrete, and technical. Do not invent numbers " +
		"that are not in the data."

	user := fmt.Sprintf(`Below is a time series of per-second load-test metrics (latencies in milliseconds).

%s

Analyze this run and answer in three clearly labeled sections:
1. WHAT'S BREAKING: the specific failure mode (e.g. latency blowup, rising error rate, throughput plateau).
2. AT WHAT LOAD: the request rate / concurrency / elapsed point where degradation begins, citing the numbers.
3. THREE FIXES: exactly three specific, actionable remediation steps, each one sentence.`, data)

	out, err := chat(system, user, false)
	if err != nil {
		return "", err
	}
	return out, nil
}

// GenerateTestPlan asks Groq to propose a load-test plan for the given base URL
// and returns it as a JSON string describing endpoints, methods, and payloads.
func GenerateTestPlan(url string) string {
	system := "You are an API load-testing expert. You output only valid JSON, " +
		"no prose, no markdown fences."

	user := fmt.Sprintf(`Given the base URL %q, propose a realistic load-test plan.

Return a JSON object with this exact shape:
{
  "baseUrl": "<the base url>",
  "endpoints": [
    {
      "path": "/example",
      "method": "GET|POST|PUT|DELETE",
      "description": "what this exercises",
      "headers": { "Header-Name": "value" },
      "payload": { }
    }
  ]
}

Suggest 4-6 endpoints that a service at this URL plausibly exposes, mixing read
and write operations. Use empty objects for payload/headers when not needed.`, url)

	out, err := chat(system, user, true)
	if err != nil {
		// Keep the contract (a JSON string) even on failure.
		errObj, _ := json.Marshal(map[string]string{"error": err.Error()})
		return string(errObj)
	}
	return out
}

// chat performs a single chat-completions call against Groq. When jsonMode is
// true it requests a JSON object response.
func chat(systemPrompt, userPrompt string, jsonMode bool) (string, error) {
	apiKey := os.Getenv("GROQ_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("GROQ_API_KEY is not set")
	}

	reqBody := chatRequest{
		Model: groqModel,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.3,
	}
	if jsonMode {
		reqBody.ResponseFormat = &responseFormat{Type: "json_object"}
	}

	buf, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, groqEndpoint, bytes.NewReader(buf))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("call groq: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("read response: %w", err)
	}

	var parsed chatResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode response (status %d): %w", resp.StatusCode, err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("groq error: %s", parsed.Error.Message)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("groq returned status %d", resp.StatusCode)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("groq returned no choices")
	}

	return parsed.Choices[0].Message.Content, nil
}
