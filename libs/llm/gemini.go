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

// GeminiClient implements the LLMProvider interface for Google Gemini API
type GeminiClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
	logger     *zap.Logger
}

// GeminiRequest represents a request to the Gemini API
type GeminiRequest struct {
	Contents          []GeminiContent         `json:"contents"`
	SystemInstruction *GeminiContent          `json:"systemInstruction,omitempty"`
	GenerationConfig  *GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

// GeminiContent represents a content object in the Gemini API
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of content (text, function call, etc.)
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiGenerationConfig contains generation parameters
type GeminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	TopP            float64 `json:"topP,omitempty"`
	TopK            int     `json:"topK,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// GeminiResponse represents a response from the Gemini API
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []GeminiPart `json:"parts"`
			Role  string       `json:"role"`
		} `json:"content"`
		FinishReason  string `json:"finishReason"`
		SafetyRatings []struct {
			Category    string `json:"category"`
			Probability string `json:"probability"`
		} `json:"safetyRatings"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
}

// NewGeminiClient creates a new Gemini API client
func NewGeminiClient(apiKey, model string, logger *zap.Logger) *GeminiClient {
	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if model == "" {
		model = "gemini-2.0-flash-exp" // Default to latest flash model
	}
	if logger == nil {
		logger, _ = zap.NewProduction()
	}

	return &GeminiClient{
		apiKey: apiKey,
		model:  model,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
		logger: logger,
	}
}

// Execute sends a prompt to Gemini and returns the response
func (g *GeminiClient) Execute(ctx context.Context, prompt string) (string, error) {
	return g.ExecuteWithSystem(ctx, prompt, "")
}

// ExecuteWithSystem sends a prompt with a system instruction to Gemini
func (g *GeminiClient) ExecuteWithSystem(ctx context.Context, prompt, systemInstruction string) (string, error) {
	g.logger.Info("Executing Gemini API request",
		zap.String("model", g.model),
		zap.Int("prompt_length", len(prompt)),
	)

	// Build request
	request := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role: "user",
				Parts: []GeminiPart{
					{Text: prompt},
				},
			},
		},
		GenerationConfig: &GeminiGenerationConfig{
			Temperature:     0.7,
			TopP:            0.95,
			MaxOutputTokens: 8192,
		},
	}

	// Add system instruction if provided
	if systemInstruction != "" {
		request.SystemInstruction = &GeminiContent{
			Parts: []GeminiPart{
				{Text: systemInstruction},
			},
		}
	}

	// Marshal request to JSON
	requestBody, err := json.Marshal(request)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Build API URL
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.model,
		g.apiKey,
	)

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(requestBody))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

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
		g.logger.Error("Gemini API error",
			zap.Int("status_code", resp.StatusCode),
			zap.String("response", string(body)),
		)
		return "", fmt.Errorf("gemini API error (status %d): %s", resp.StatusCode, string(body))
	}

	// Parse response
	var geminiResp GeminiResponse
	if err := json.Unmarshal(body, &geminiResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract text from response
	if len(geminiResp.Candidates) == 0 {
		return "", fmt.Errorf("no candidates in response")
	}

	candidate := geminiResp.Candidates[0]
	if len(candidate.Content.Parts) == 0 {
		return "", fmt.Errorf("no parts in candidate content")
	}

	responseText := candidate.Content.Parts[0].Text

	// Log metrics
	duration := time.Since(startTime)
	g.logger.Info("Gemini API request completed",
		zap.Duration("duration", duration),
		zap.Int("prompt_tokens", geminiResp.UsageMetadata.PromptTokenCount),
		zap.Int("completion_tokens", geminiResp.UsageMetadata.CandidatesTokenCount),
		zap.Int("total_tokens", geminiResp.UsageMetadata.TotalTokenCount),
		zap.String("finish_reason", candidate.FinishReason),
		zap.Int("response_length", len(responseText)),
	)

	return responseText, nil
}

// DecomposeTask uses Gemini to break down a complex task into steps
func (g *GeminiClient) DecomposeTask(ctx context.Context, userPrompt string, availableAgents []string) (*TaskPlan, error) {
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

	// Call Gemini with system instruction
	response, err := g.ExecuteWithSystem(ctx, prompt, systemInstruction)
	if err != nil {
		return nil, fmt.Errorf("failed to get task decomposition: %w", err)
	}

	// Parse JSON response
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
func (g *GeminiClient) GetModel() string {
	return g.model
}

// SetModel changes the model to use
func (g *GeminiClient) SetModel(model string) {
	g.model = model
}
