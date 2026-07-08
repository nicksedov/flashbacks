package llm

import (
	"context"
	"encoding/json"
)

// ChatMessage represents a single message in a conversation.
type ChatMessage struct {
	Role       string     `json:"role"` // "system", "user", "assistant", "tool"
	Content    string     `json:"content"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`   // populated when assistant requests tool invocations
	ToolCallID string     `json:"tool_call_id,omitempty"` // populated for role "tool" responses
}

// ToolCall represents a single tool invocation requested by the LLM.
type ToolCall struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

// ToolDefinition describes an available tool to the LLM using JSON Schema for parameters.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters"` // JSON Schema object
}

// ChatRequest is the input for ChatClient.Chat.
type ChatRequest struct {
	Messages []ChatMessage
	Tools    []ToolDefinition
}

// ChatUsage carries token usage information from an LLM chat response.
// DeepSeek-specific extended fields (PromptCacheHitTokens, PromptCacheMissTokens,
// ReasoningTokens) are populated only when the provider returns them.
type ChatUsage struct {
	PromptTokens          int `json:"promptTokens"`
	CompletionTokens      int `json:"completionTokens"`
	TotalTokens           int `json:"totalTokens"`
	PromptCacheHitTokens  int `json:"promptCacheHitTokens,omitempty"`  // DeepSeek: prompt_cache_hit_tokens
	PromptCacheMissTokens int `json:"promptCacheMissTokens,omitempty"` // DeepSeek: prompt_cache_miss_tokens
	ReasoningTokens       int `json:"reasoningTokens,omitempty"`       // DeepSeek: completion_tokens_details.reasoning_tokens
}

// ChatResponse is the output from ChatClient.Chat.
type ChatResponse struct {
	Message    ChatMessage
	StopReason string     // "end_turn" or "tool_use"
	Usage      *ChatUsage // Token usage from the API response (nil if not provided)
}

// ChatClient extends Client with conversational capabilities including tool use.
type ChatClient interface {
	Client
	Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error)
}

// NewChatClient wraps a Client into a ChatClient if the underlying implementation supports it,
// otherwise returns nil, false.
func NewChatClient(c Client) (ChatClient, bool) {
	if cc, ok := c.(ChatClient); ok {
		return cc, true
	}
	return nil, false
}
