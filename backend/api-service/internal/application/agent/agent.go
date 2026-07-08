package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"
)

// AgentConfig holds agent configuration.
type AgentConfig struct {
	MaxTokens             int // Token threshold for summarization (default 8000)
	MaxToolRounds         int // Maximum tool-use iterations per message (default 10)
	MaxConversationTokens int // Max tokens per conversation before exhaustion (default 128000)
}

// DefaultAgentConfig returns sensible defaults.
func DefaultAgentConfig() AgentConfig {
	return AgentConfig{
		MaxTokens:             8000,
		MaxToolRounds:         10,
		MaxConversationTokens: 128000,
	}
}

// ToolCallInfo describes a tool invocation for the frontend.
type ToolCallInfo struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
	Result    string `json:"result"`
}

// AgentResponse is the result of processing a user message.
type AgentResponse struct {
	Message   string         `json:"message"`
	ToolCalls []ToolCallInfo `json:"toolCalls,omitempty"`
}

// ToolEvent represents a real-time tool execution event streamed to the frontend.
type ToolEvent struct {
	Type       string `json:"type"` // "tool_call", "tool_result", "message", "error", "done", "token_usage"
	Name       string `json:"name,omitempty"`
	Status     string `json:"status,omitempty"`
	Result     string `json:"result,omitempty"`
	Content    string `json:"content,omitempty"`
	Error      string `json:"error,omitempty"`
	TokenCount int    `json:"tokenCount,omitempty"`
	MaxTokens  int    `json:"maxTokens,omitempty"`

	// DeepSeek-specific extended usage fields (populated when provider returns them)
	PromptTokens          int `json:"promptTokens,omitempty"`
	CompletionTokens      int `json:"completionTokens,omitempty"`
	PromptCacheHitTokens  int `json:"promptCacheHitTokens,omitempty"`
	PromptCacheMissTokens int `json:"promptCacheMissTokens,omitempty"`
	ReasoningTokens       int `json:"reasoningTokens,omitempty"`
}

// ToolEventHandler is called during agent execution to stream events.
type ToolEventHandler func(event ToolEvent)

// ToolProvider supplies tool definitions and execution to the agent.
// Implemented by mcpserver.FlashbacksMCPServer.
type ToolProvider interface {
	ToolDefinitions() []llm.ToolDefinition
	ExecuteTool(ctx context.Context, name string, arguments json.RawMessage) (string, error)
}

// Agent orchestrates conversational AI with MCP tool invocations.
type Agent struct {
	conversationService *ConversationService
	toolProvider        ToolProvider
	config              AgentConfig
	summaryMu           sync.Mutex
	summaryInFlight     map[uint]bool // tracks convIDs with summary generation in progress
}

// NewAgent creates a new agent instance.
func NewAgent(convService *ConversationService, toolProvider ToolProvider, config AgentConfig) *Agent {
	return &Agent{
		conversationService: convService,
		toolProvider:        toolProvider,
		config:              config,
		summaryInFlight:     make(map[uint]bool),
	}
}

// ProcessMessage handles a user message, runs the agent loop, and returns the assistant response.
// If eventHandler is non-nil, it receives real-time events during processing.
// maxTokens: 0 = use config default; >0 = override max tokens for this call.
func (a *Agent) ProcessMessage(ctx context.Context, convID uint, userMessage string, chatClient llm.ChatClient, eventHandler ToolEventHandler, maxTokens int) (*AgentResponse, error) {
	// Resolve effective max tokens
	effectiveMax := a.config.MaxConversationTokens
	if maxTokens > 0 {
		effectiveMax = maxTokens
	}

	// Check token exhaustion before processing
	conv, err := a.conversationService.GetConversationByID(ctx, convID)
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}
	if effectiveMax > 0 && conv.TokenCount >= effectiveMax {
		if eventHandler != nil {
			eventHandler(ToolEvent{Type: "error", Error: "Token limit reached. Start a new conversation to continue."})
			eventHandler(ToolEvent{Type: "token_usage", TokenCount: conv.TokenCount, MaxTokens: effectiveMax})
			eventHandler(ToolEvent{Type: "done"})
		}
		return &AgentResponse{Message: "Token limit reached."}, nil
	}

	// Save user message
	userMsg := domain.ConversationMessage{
		Role:       "user",
		Content:    userMessage,
		TokenCount: estimateTokens(userMessage),
	}
	if err := a.conversationService.AddMessage(ctx, convID, userMsg); err != nil {
		return nil, fmt.Errorf("failed to save user message: %w", err)
	}

	// Reload conversation to get updated context
	conv, err = a.conversationService.GetConversationByID(ctx, convID)
	if err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}

	// Load message history
	messages, err := a.conversationService.GetMessages(ctx, convID)
	if err != nil {
		return nil, fmt.Errorf("failed to load message history: %w", err)
	}

	// Build system prompt
	systemPrompt := a.buildSystemPrompt(conv)

	// Get tool definitions from tool provider
	toolDefs := a.toolProvider.ToolDefinitions()

	// Convert to chat messages
	chatMessages := MessagesToChatMessages(messages)

	// Prepend system prompt
	fullMessages := append([]llm.ChatMessage{{Role: "system", Content: systemPrompt}}, chatMessages...)

	var allToolCalls []ToolCallInfo

	// Agent loop: iterate until the LLM produces a final text response or max rounds exceeded
	for round := 0; round < a.config.MaxToolRounds; round++ {
		resp, err := chatClient.Chat(ctx, llm.ChatRequest{
			Messages: fullMessages,
			Tools:    toolDefs,
		})
		if err != nil {
			if eventHandler != nil {
				eventHandler(ToolEvent{Type: "error", Error: err.Error()})
			}
			return nil, fmt.Errorf("LLM chat failed: %w", err)
		}

		if resp.StopReason == "end_turn" || len(resp.Message.ToolCalls) == 0 {
			// Final text response
			assistantMsg := domain.ConversationMessage{
				Role:       "assistant",
				Content:    resp.Message.Content,
				TokenCount: estimateTokens(resp.Message.Content),
			}
			if err := a.conversationService.AddMessage(ctx, convID, assistantMsg); err != nil {
				log.Printf("Failed to save assistant message: %v", err)
			}

			if eventHandler != nil {
				eventHandler(ToolEvent{Type: "message", Content: resp.Message.Content})
				// Emit token usage event with optional DeepSeek-specific fields
				tokenCount, _ := a.conversationService.CountTokens(ctx, convID)
				usageEvent := ToolEvent{
					Type:       "token_usage",
					TokenCount: tokenCount,
					MaxTokens:  effectiveMax,
				}
				if resp.Usage != nil {
					usageEvent.PromptTokens = resp.Usage.PromptTokens
					usageEvent.CompletionTokens = resp.Usage.CompletionTokens
					usageEvent.PromptCacheHitTokens = resp.Usage.PromptCacheHitTokens
					usageEvent.PromptCacheMissTokens = resp.Usage.PromptCacheMissTokens
					usageEvent.ReasoningTokens = resp.Usage.ReasoningTokens
				}
				eventHandler(usageEvent)
				eventHandler(ToolEvent{Type: "done"})
			}

			// Check token threshold and summarize if needed
			a.maybeSummarize(ctx, convID, chatClient)

			// Trigger summary generation in background if needed
			a.maybeGenerateSummary(ctx, convID, chatClient)

			return &AgentResponse{
				Message:   resp.Message.Content,
				ToolCalls: allToolCalls,
			}, nil
		}

		// Tool use: execute each tool and continue the loop
		// Save assistant message with tool calls
		toolCallsJSON, _ := json.Marshal(resp.Message.ToolCalls)
		assistantMsg := domain.ConversationMessage{
			Role:          "assistant",
			Content:       resp.Message.Content,
			ToolCallsJSON: string(toolCallsJSON),
			TokenCount:    estimateTokens(resp.Message.Content),
		}
		if err := a.conversationService.AddMessage(ctx, convID, assistantMsg); err != nil {
			log.Printf("Failed to save assistant tool_call message: %v", err)
		}

		// Add assistant message with tool calls to fullMessages
		fullMessages = append(fullMessages, resp.Message)

		// Execute each tool call
		for _, tc := range resp.Message.ToolCalls {
			if eventHandler != nil {
				eventHandler(ToolEvent{
					Type:   "tool_call",
					Name:   tc.Name,
					Status: "running",
				})
			}

			result, execErr := a.toolProvider.ExecuteTool(ctx, tc.Name, tc.Arguments)

			if execErr != nil {
				result = fmt.Sprintf("Error: %s", execErr.Error())
			}

			if eventHandler != nil {
				eventHandler(ToolEvent{
					Type:   "tool_result",
					Name:   tc.Name,
					Status: "completed",
					Result: result,
				})
			}

			allToolCalls = append(allToolCalls, ToolCallInfo{
				Name:      tc.Name,
				Arguments: string(tc.Arguments),
				Result:    result,
			})

			// Save tool result message
			toolResultMsg := domain.ConversationMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
				TokenCount: estimateTokens(result),
			}
			if err := a.conversationService.AddMessage(ctx, convID, toolResultMsg); err != nil {
				log.Printf("Failed to save tool result: %v", err)
			}

			// Add tool result to fullMessages for the next LLM call
			fullMessages = append(fullMessages, llm.ChatMessage{
				Role:       "tool",
				Content:    result,
				ToolCallID: tc.ID,
			})
		}
	}

	// Max rounds exceeded
	fallbackMsg := "I've reached the maximum number of tool invocations. Here's what I found so far."
	if eventHandler != nil {
		eventHandler(ToolEvent{Type: "message", Content: fallbackMsg})
		tokenCount, _ := a.conversationService.CountTokens(ctx, convID)
		eventHandler(ToolEvent{Type: "token_usage", TokenCount: tokenCount, MaxTokens: effectiveMax})
		eventHandler(ToolEvent{Type: "done"})
	}

	// Trigger summary generation in background if needed
	a.maybeGenerateSummary(ctx, convID, chatClient)

	return &AgentResponse{
		Message:   fallbackMsg,
		ToolCalls: allToolCalls,
	}, nil
}

// buildSystemPrompt creates the system prompt for the agent based on conversation context.
func (a *Agent) buildSystemPrompt(conv *domain.Conversation) string {
	prompt := `You are an AI assistant for the Flashbacks application. You help users analyze, search, and understand their image collection.

You have access to the following tools:
` + a.buildToolList() + `

Guidelines:
- Use tools when you need information. Don't guess.
- Be helpful, specific, and accurate.
- When presenting search results, list images with their paths and key metadata.`

	// Add language instruction based on conversation language
	switch conv.Language {
	case "ru":
		prompt += "\n- ВАЖНО: Всегда отвечайте исключительно на русском языке. Все описания, ответы и пояснения должны быть на русском."
	default:
		prompt += "\n- Always respond in English unless the user explicitly writes in another language."
	}

	if conv.ImagePath != "" {
		prompt += fmt.Sprintf("\n\nWhen the user refers to \"this image\" or \"the image\", they mean: %s", conv.ImagePath)
	}

	return prompt
}

// buildToolList generates a dynamic list of available tools from the ToolProvider.
func (a *Agent) buildToolList() string {
	toolDefs := a.toolProvider.ToolDefinitions()
	if len(toolDefs) == 0 {
		return "(no tools available)"
	}

	var b strings.Builder
	for _, td := range toolDefs {
		b.WriteString(fmt.Sprintf("- %s: %s\n", td.Name, td.Description))
	}
	return b.String()
}

// maybeSummarize checks token count and triggers summarization if threshold exceeded.
func (a *Agent) maybeSummarize(ctx context.Context, convID uint, chatClient llm.ChatClient) {
	tokenCount, err := a.conversationService.CountTokens(ctx, convID)
	if err != nil {
		log.Printf("Failed to count tokens for conversation %d: %v", convID, err)
		return
	}

	if tokenCount > a.config.MaxTokens {
		log.Printf("Conversation %d has %d tokens (threshold %d), summarizing...", convID, tokenCount, a.config.MaxTokens)
		if err := a.conversationService.SummarizeOlderMessages(ctx, convID, 6, chatClient); err != nil {
			log.Printf("Failed to summarize conversation %d: %v", convID, err)
		}
	}
}

// maybeGenerateSummary triggers summary generation in a goroutine if summary is empty
// and there are at least 2 user messages. Prevents duplicate goroutines for the same conversation.
func (a *Agent) maybeGenerateSummary(ctx context.Context, convID uint, chatClient llm.ChatClient) {
	// Check if summary generation is already in flight for this conversation
	a.summaryMu.Lock()
	if a.summaryInFlight[convID] {
		a.summaryMu.Unlock()
		return
	}
	a.summaryMu.Unlock()

	conv, err := a.conversationService.GetConversationByID(ctx, convID)
	if err != nil || conv.Summary != "" {
		return
	}

	// Count user messages
	messages, err := a.conversationService.GetMessages(ctx, convID)
	if err != nil {
		return
	}
	userMsgCount := 0
	for _, m := range messages {
		if m.Role == "user" {
			userMsgCount++
		}
	}
	if userMsgCount >= 2 {
		a.summaryMu.Lock()
		if a.summaryInFlight[convID] {
			a.summaryMu.Unlock()
			return
		}
		a.summaryInFlight[convID] = true
		a.summaryMu.Unlock()

		go func() {
			defer func() {
				a.summaryMu.Lock()
				delete(a.summaryInFlight, convID)
				a.summaryMu.Unlock()
			}()
			a.conversationService.GenerateDisplaySummary(context.Background(), convID, chatClient)
		}()
	}
}
