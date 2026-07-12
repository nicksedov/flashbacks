package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"

	"gorm.io/gorm"
)

// ConversationService manages conversation persistence and context compression.
type ConversationService struct {
	db *gorm.DB
}

// NewConversationService creates a new conversation service.
func NewConversationService(db *gorm.DB) *ConversationService {
	return &ConversationService{db: db}
}

// CreateConversation creates a new conversation for a user.
func (s *ConversationService) CreateConversation(ctx context.Context, userID uint, imagePath, language string) (*domain.Conversation, error) {
	if language == "" {
		language = "en"
	}
	conv := &domain.Conversation{
		UserID:    userID,
		ImagePath: imagePath,
		Title:     "New Chat",
		Language:  language,
	}
	if err := s.db.Create(conv).Error; err != nil {
		return nil, fmt.Errorf("failed to create conversation: %w", err)
	}
	return conv, nil
}

// GetConversation retrieves a conversation by ID, verifying user ownership.
func (s *ConversationService) GetConversation(ctx context.Context, convID, userID uint) (*domain.Conversation, error) {
	var conv domain.Conversation
	if err := s.db.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}
	return &conv, nil
}

// GetConversationByID retrieves a conversation by ID without user ownership check.
// Use this for internal/agent use where userID is already validated.
func (s *ConversationService) GetConversationByID(ctx context.Context, convID uint) (*domain.Conversation, error) {
	var conv domain.Conversation
	if err := s.db.First(&conv, convID).Error; err != nil {
		return nil, fmt.Errorf("conversation not found: %w", err)
	}
	return &conv, nil
}

// ListConversations returns all conversations for a user that have at least one message, ordered by most recent.
func (s *ConversationService) ListConversations(ctx context.Context, userID uint) ([]domain.Conversation, error) {
	var conversations []domain.Conversation
	if err := s.db.Where("user_id = ? AND token_count > 0", userID).Order("updated_at DESC").Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("failed to list conversations: %w", err)
	}
	return conversations, nil
}

// ListConversationsByImage returns conversations for a user filtered by image path.
// Only returns conversations that have at least one message.
func (s *ConversationService) ListConversationsByImage(ctx context.Context, userID uint, imagePath string) ([]domain.Conversation, error) {
	var conversations []domain.Conversation
	if err := s.db.Where("user_id = ? AND image_path = ? AND token_count > 0", userID, imagePath).Order("updated_at DESC").Find(&conversations).Error; err != nil {
		return nil, fmt.Errorf("failed to list conversations by image: %w", err)
	}
	return conversations, nil
}

// DeleteConversation deletes a conversation and all its messages.
func (s *ConversationService) DeleteConversation(ctx context.Context, convID, userID uint) error {
	// Verify ownership
	var conv domain.Conversation
	if err := s.db.Where("id = ? AND user_id = ?", convID, userID).First(&conv).Error; err != nil {
		return fmt.Errorf("conversation not found: %w", err)
	}

	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("conversation_id = ?", convID).Delete(&domain.ConversationMessage{}).Error; err != nil {
			return fmt.Errorf("failed to delete messages: %w", err)
		}
		if err := tx.Delete(&conv).Error; err != nil {
			return fmt.Errorf("failed to delete conversation: %w", err)
		}
		return nil
	})
}

// AddMessage adds a message to a conversation and updates the conversation timestamp and token count.
func (s *ConversationService) AddMessage(ctx context.Context, convID uint, msg domain.ConversationMessage) error {
	msg.ConversationID = convID
	// Auto-estimate token count if not provided
	if msg.TokenCount == 0 && msg.Content != "" {
		msg.TokenCount = estimateTokens(msg.Content)
	}
	if err := s.db.Create(&msg).Error; err != nil {
		return fmt.Errorf("failed to add message: %w", err)
	}

	// Update conversation timestamp, title (if first user message), and token count
	updates := map[string]interface{}{
		"updated_at":  msg.CreatedAt,
		"token_count": gorm.Expr("token_count + ?", msg.TokenCount),
	}
	s.db.Model(&domain.Conversation{}).Where("id = ?", convID).Updates(updates)

	// Auto-generate title from first user message
	var count int64
	s.db.Model(&domain.ConversationMessage{}).
		Where("conversation_id = ? AND role = ?", convID, "user").
		Count(&count)

	if count == 1 && msg.Role == "user" {
		title := truncateUTF8(msg.Content, 50)
		if err := s.db.Model(&domain.Conversation{}).
			Where("id = ?", convID).
			Update("title", title).Error; err != nil {
			return fmt.Errorf("failed to update conversation title: %w", err)
		}
	}

	return nil
}

// CleanupEmptyConversations deletes conversations with no messages for a given user and image path.
// Used to prevent accumulating empty conversation rows when the user opens the chat panel.
func (s *ConversationService) CleanupEmptyConversations(ctx context.Context, userID uint, imagePath string) {
	s.db.Where("user_id = ? AND image_path = ? AND token_count = 0", userID, imagePath).
		Delete(&domain.Conversation{})
}

// GetMessages returns all messages for a conversation, ordered chronologically.
func (s *ConversationService) GetMessages(ctx context.Context, convID uint) ([]domain.ConversationMessage, error) {
	var messages []domain.ConversationMessage
	if err := s.db.Where("conversation_id = ?", convID).Order("created_at ASC, id ASC").Find(&messages).Error; err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	return messages, nil
}

// CountTokens returns the cached token count for a conversation.
func (s *ConversationService) CountTokens(ctx context.Context, convID uint) (int, error) {
	var conv domain.Conversation
	if err := s.db.Select("token_count").First(&conv, convID).Error; err != nil {
		return 0, fmt.Errorf("failed to count tokens: %w", err)
	}
	return conv.TokenCount, nil
}

// SummarizeOlderMessages compresses older messages into a summary to reduce context size.
// Keeps the last `keepRecent` messages intact and summarizes everything before them.
// Adjusts the split point to avoid breaking tool-call/tool-result message pairs:
// a "tool" role message must always be preceded by an "assistant" message with matching tool_calls.
func (s *ConversationService) SummarizeOlderMessages(ctx context.Context, convID uint, keepRecent int, chatClient llm.ChatClient) error {
	var messages []domain.ConversationMessage
	if err := s.db.Where("conversation_id = ?", convID).Order("created_at ASC, id ASC").Find(&messages).Error; err != nil {
		return fmt.Errorf("failed to get messages: %w", err)
	}

	if len(messages) <= keepRecent {
		return nil // Nothing to summarize
	}

	// Find a safe split point that doesn't break tool-call/tool-result pairs.
	// Start from the default split and walk backward if needed.
	splitIdx := len(messages) - keepRecent
	splitIdx = findSafeSummarizeSplit(messages, splitIdx)

	if splitIdx <= 0 {
		return nil // Cannot safely summarize anything
	}

	toSummarize := messages[:splitIdx]
	toKeep := messages[splitIdx:]

	// Build text to summarize
	var sb strings.Builder
	for _, msg := range toSummarize {
		fmt.Fprintf(&sb, "[%s]: %s\n", msg.Role, msg.Content)
	}

	// Call LLM to summarize
	summary, err := chatClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{
				Role:    "system",
				Content: "Summarize the following conversation concisely, preserving key information, tool results, and context. Output only the summary text.",
			},
			{
				Role:    "user",
				Content: sb.String(),
			},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to summarize conversation: %w", err)
	}

	// Delete old messages and replace with a single summary message
	return s.db.Transaction(func(tx *gorm.DB) error {
		// Delete old messages
		var oldIDs []uint
		for _, msg := range toSummarize {
			oldIDs = append(oldIDs, msg.ID)
		}
		if len(oldIDs) > 0 {
			if err := tx.Where("id IN ?", oldIDs).Delete(&domain.ConversationMessage{}).Error; err != nil {
				return fmt.Errorf("failed to delete old messages: %w", err)
			}
		}

		// Insert summary message at the beginning
		summaryMsg := domain.ConversationMessage{
			ConversationID: convID,
			Role:           "system",
			Content:        "[Previous conversation summary]: " + summary.Message.Content,
			TokenCount:     estimateTokens(summary.Message.Content),
		}
		// Set created_at to before the kept messages
		if len(toKeep) > 0 {
			summaryMsg.CreatedAt = toKeep[0].CreatedAt.Add(-1)
		}
		if err := tx.Create(&summaryMsg).Error; err != nil {
			return fmt.Errorf("failed to create summary message: %w", err)
		}

		return nil
	})
}

// ResolveModelMaxTokens reads LlmProviderModelCache for the given provider, finds model by name,
// and returns its ContextLength. Returns 0 if not found/unavailable.
func (s *ConversationService) ResolveModelMaxTokens(providerAlias, modelName string) int {
	var cache domain.LlmProviderModelCache
	if err := s.db.Where("provider_alias = ?", providerAlias).First(&cache).Error; err != nil {
		return 0
	}

	var models []llm.ModelInfo
	if err := json.Unmarshal([]byte(cache.ModelsJSON), &models); err != nil {
		return 0
	}

	for _, m := range models {
		if m.Name == modelName || m.ID == modelName {
			return m.ContextLength
		}
	}
	return 0
}

// GenerateDisplaySummary generates an LLM summary for conversation history.
// Falls back to first user message if fewer than 2 messages.
func (s *ConversationService) GenerateDisplaySummary(ctx context.Context, convID uint, chatClient llm.ChatClient) {
	conv, err := s.GetConversationByID(ctx, convID)
	if err != nil {
		return
	}
	// Skip if already has summary
	if conv.Summary != "" {
		return
	}

	messages, err := s.GetMessages(ctx, convID)
	if err != nil || len(messages) < 2 {
		// Fallback: use first user message as summary
		for _, m := range messages {
			if m.Role == "user" {
				summary := truncateUTF8(m.Content, 100)
				s.db.Model(&domain.Conversation{}).Where("id = ?", convID).Update("summary", summary)
				return
			}
		}
		return
	}

	// Build conversation text for summarization
	var sb strings.Builder
	for _, m := range messages {
		if m.Role == "user" || m.Role == "assistant" {
			fmt.Fprintf(&sb, "[%s]: %s\n", m.Role, m.Content)
		}
	}

	resp, err := chatClient.Chat(ctx, llm.ChatRequest{
		Messages: []llm.ChatMessage{
			{Role: "system", Content: "Generate a brief one-line summary (max 80 chars) of this conversation. Output only the summary text, no quotes."},
			{Role: "user", Content: sb.String()},
		},
	})
	if err != nil {
		log.Printf("Failed to generate summary for conversation %d: %v", convID, err)
		// Fallback to first user message
		for _, m := range messages {
			if m.Role == "user" {
				summary := truncateUTF8(m.Content, 100)
				s.db.Model(&domain.Conversation{}).Where("id = ?", convID).Update("summary", summary)
				return
			}
		}
		return
	}

	summary := truncateUTF8(resp.Message.Content, 100)
	s.db.Model(&domain.Conversation{}).Where("id = ?", convID).Update("summary", summary)
}

// MessagesToChatMessages converts domain messages to LLM chat messages.
func MessagesToChatMessages(messages []domain.ConversationMessage) []llm.ChatMessage {
	var result []llm.ChatMessage
	for _, msg := range messages {
		cm := llm.ChatMessage{
			Role:       msg.Role,
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}

		// Restore tool calls from JSON if present
		if msg.ToolCallsJSON != "" {
			var toolCalls []llm.ToolCall
			if err := json.Unmarshal([]byte(msg.ToolCallsJSON), &toolCalls); err == nil {
				cm.ToolCalls = toolCalls
			}
		}

		result = append(result, cm)
	}
	return result
}

// findSafeSummarizeSplit walks backward from the proposed split index to find a
// boundary that does not break tool-call/tool-result pairs. A "tool" message
// must always have its corresponding "assistant" message with matching
// tool_calls on the same side of the split.
func findSafeSummarizeSplit(messages []domain.ConversationMessage, splitIdx int) int {
	if splitIdx <= 0 || splitIdx >= len(messages) {
		return splitIdx
	}

	// Collect tool_call IDs that appear in the "keep" (right) portion.
	// If any of those IDs were produced by an assistant message in the
	// "summarize" (left) portion, we must move the split left to include
	// that assistant message (and all subsequent messages up to the tool result).
	keepToolIDs := make(map[string]bool)
	for i := splitIdx; i < len(messages); i++ {
		if messages[i].Role == "tool" && messages[i].ToolCallID != "" {
			keepToolIDs[messages[i].ToolCallID] = true
		}
	}

	if len(keepToolIDs) == 0 {
		return splitIdx // No orphan risk
	}

	// Find the earliest assistant message in the "toSummarize" portion whose
	// tool_call IDs appear in the "toKeep" portion. Move the split to include
	// that assistant and everything after it.
	for i := splitIdx - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" && messages[i].ToolCallsJSON != "" {
			var toolCalls []llm.ToolCall
			if err := json.Unmarshal([]byte(messages[i].ToolCallsJSON), &toolCalls); err == nil {
				for _, tc := range toolCalls {
					if keepToolIDs[tc.ID] {
						// This assistant produced a tool call whose result is in the keep portion.
						// Move split to include this assistant message.
						return i
					}
				}
			}
		}
	}

	return splitIdx
}

// SanitizeChatMessages removes or fixes messages that would cause API errors.
// Specifically, it ensures every "tool" role message has a preceding
// "assistant" message with a matching tool_call ID. Orphan tool messages
// (those without a matching preceding assistant tool_call) are converted
// to "system" role messages so they don't break the API contract.
func SanitizeChatMessages(messages []llm.ChatMessage) []llm.ChatMessage {
	// First pass: collect all tool_call IDs from assistant messages.
	toolCallIDs := make(map[string]bool)
	for _, m := range messages {
		if m.Role == "assistant" {
			for _, tc := range m.ToolCalls {
				toolCallIDs[tc.ID] = true
			}
		}
	}

	// Second pass: fix orphan tool messages.
	cleaned := make([]llm.ChatMessage, 0, len(messages))
	for _, m := range messages {
		if m.Role == "tool" {
			if m.ToolCallID == "" || !toolCallIDs[m.ToolCallID] {
				// Orphan tool message: convert to system to preserve context
				// without violating the API contract.
				cleaned = append(cleaned, llm.ChatMessage{
					Role:    "system",
					Content: "[orphan tool result]: " + m.Content,
				})
				continue
			}
		}
		cleaned = append(cleaned, m)
	}

	return cleaned
}

// truncateUTF8 truncates a string to a maximum number of runes (not bytes),
// appending "..." if truncated. This ensures valid UTF-8 is never broken
// mid-character.
func truncateUTF8(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return s
}

// estimateTokens provides a rough token count estimate.
func estimateTokens(text string) int {
	// Count Cyrillic vs Latin to adjust estimate
	cyrillicCount := 0
	totalChars := 0
	for _, ch := range text {
		totalChars++
		if ch >= 0x0400 && ch <= 0x04FF {
			cyrillicCount++
		}
	}

	if totalChars == 0 {
		return 0
	}

	// If mostly Cyrillic, use ~2 chars per token; otherwise ~4 chars per token
	if float64(cyrillicCount)/float64(totalChars) > 0.5 {
		return (totalChars + 1) / 2
	}
	return (totalChars + 3) / 4
}
