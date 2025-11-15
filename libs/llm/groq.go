package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"go.uber.org/zap"
)

// GroqClient implements the LLMProvider interface for Groq API
type GroqClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
	logger     *zap.Logger
}

// GroqRequest represents a request to the Groq API (OpenAI-compatible)
type GroqRequest struct {
	Model       string        `json:"model"`
	Messages    []GroqMessage `json:"messages"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
}

// GroqMessage represents a message in the conversation
type GroqMessage struct {
	Role    string `json:"role"` // "system", "user", or "assistant"
	Content string `json:"content"`
}

// GroqResponse represents a response from the Groq API
type GroqResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// NewGroqClient creates a new Groq API client
func NewGroqClient(apiKey, model string, logger *zap.Logger) *GroqClient {
	if apiKey == "" {
		apiKey = os.Getenv("GROQ_API_KEY")
	}
	if model == "" {
		model = "meta-llama/llama-4-scout-17b-16e-instruct" // Default to fast Llama 4
	}
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	return &GroqClient{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

// Execute sends a prompt to Groq and returns the response
func (g *GroqClient) Execute(ctx context.Context, prompt string) (string, error) {
	return g.ExecuteWithSystem(ctx, prompt, "")
}

// ExecuteWithSystem sends a prompt with a system instruction to Groq
func (g *GroqClient) ExecuteWithSystem(ctx context.Context, prompt, systemInstruction string) (string, error) {
	g.logger.Info("Executing Groq API request",
		zap.String("model", g.model),
		zap.Int("prompt_length", len(prompt)),
	)

	// Build messages array
	messages := []GroqMessage{}

	if systemInstruction != "" {
		messages = append(messages, GroqMessage{
			Role:    "system",
			Content: systemInstruction,
		})
	}

	messages = append(messages, GroqMessage{
		Role:    "user",
		Content: prompt,
	})

	// Build request
	request := GroqRequest{
		Model:       g.model,
		Messages:    messages,
		Temperature: 0.7,
		MaxTokens:   8192,
	}

	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := "https://api.groq.com/openai/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", g.apiKey))

	// Send request
	startTime := time.Now()
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}

	// Check for HTTP errors
	if resp.StatusCode != http.StatusOK {
		g.logger.Error("Groq API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
		)
		return "", fmt.Errorf("groq API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var groqResp GroqResponse
	if err := json.Unmarshal(body, &groqResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract text from response
	if len(groqResp.Choices) == 0 {
		return "", fmt.Errorf("no choices in response")
	}

	responseText := groqResp.Choices[0].Message.Content

	// Log metrics
	duration := time.Since(startTime)
	tokensPerSecond := float64(groqResp.Usage.CompletionTokens) / duration.Seconds()

	g.logger.Info("Groq API request completed",
		zap.Duration("duration", duration),
		zap.Int("prompt_tokens", groqResp.Usage.PromptTokens),
		zap.Int("completion_tokens", groqResp.Usage.CompletionTokens),
		zap.Int("total_tokens", groqResp.Usage.TotalTokens),
		zap.Float64("tokens_per_second", tokensPerSecond),
		zap.String("finish_reason", groqResp.Choices[0].FinishReason),
		zap.Int("response_length", len(responseText)),
	)

	return responseText, nil
}

// DecomposeTask uses Groq to break down a complex task into steps
func (g *GroqClient) DecomposeTask(ctx context.Context, userPrompt string, availableAgents []string) (*TaskPlan, error) {
	systemInstruction := `You are an intelligent task orchestrator for the Ainur agent platform.

Your role is to decompose complex user tasks into a sequential plan of steps that can be executed by specialized WASM agents.

Return your response as a JSON object with this exact structure:
{
  "plan": [
    {
      "step": 1,
      "description": "Clear description of what this step does",
      "agent": "agent-name",
      "function": "function_name",
      "args": ["arg1", "arg2"]
    }
  ]
}

Rules:
1. Each step must use an available agent from the list provided
2. Steps execute sequentially - later steps can use results from earlier steps
3. For final summarization or text generation, use agent "llm" with function "generate"
4. Keep plans simple and efficient - don't over-complicate
5. Return ONLY valid JSON, no explanation or markdown formatting`

	prompt := fmt.Sprintf(`Available agents:
%v

User task:
%s

Create a sequential execution plan.`, availableAgents, userPrompt)

	g.logger.Info("Decomposing task",
		zap.String("user_prompt", userPrompt),
		zap.Strings("available_agents", availableAgents),
	)

	// Call Groq with system instruction
	response, err := g.ExecuteWithSystem(ctx, prompt, systemInstruction)
	if err != nil {
		return nil, fmt.Errorf("failed to get task decomposition: %w", err)
	}

	// Parse JSON response (Groq sometimes wraps in markdown code blocks)
	response = cleanJSONResponse(response)

	var plan TaskPlan
	if err := json.Unmarshal([]byte(response), &plan); err != nil {
		g.logger.Error("Failed to parse task plan JSON",
			zap.Error(err),
			zap.String("response", response),
		)
		return nil, fmt.Errorf("failed to parse task plan: %w", err)
	}

	g.logger.Info("Task decomposed successfully",
		zap.Int("steps", len(plan.Steps)),
	)

	return &plan, nil
}

// GetModel returns the current model name
func (g *GroqClient) GetModel() string {
	return g.model
}

// SetModel changes the model to use
func (g *GroqClient) SetModel(model string) {
	g.model = model
}

// cleanJSONResponse removes markdown code blocks if present
func cleanJSONResponse(response string) string {
	// Remove ```json ... ``` if present
	if len(response) > 7 && response[:7] == "```json" {
		response = response[7:]
		if idx := len(response) - 3; idx > 0 && response[idx:] == "```" {
			response = response[:idx]
		}
	} else if len(response) > 3 && response[:3] == "```" {
		response = response[3:]
		if idx := len(response) - 3; idx > 0 && response[idx:] == "```" {
			response = response[:idx]
		}
	}
	return response
}
