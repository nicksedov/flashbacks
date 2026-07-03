package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/flashbacks/api-service/internal/application/agent"
	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"
	"github.com/flashbacks/api-service/internal/interfaces/dto"
	"github.com/flashbacks/api-service/internal/interfaces/i18n"
	"github.com/flashbacks/api-service/internal/interfaces/middleware"

	"github.com/gin-gonic/gin"
)

// handleCreateConversation handles POST /api/chat/conversations
func (s *Server) handleCreateConversation(c *gin.Context) {
	var req dto.CreateConversationRequest
	if err := c.ShouldBindJSON(&req); err != nil && err != io.EOF {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgChatInvalidRequest)
		return
	}

	userID := middleware.GetUserID(c)

	// Clean up any previous empty conversations for this image before creating a new one
	if req.ImagePath != "" {
		s.conversationService.CleanupEmptyConversations(c.Request.Context(), userID, req.ImagePath)
	}

	conv, err := s.conversationService.CreateConversation(c.Request.Context(), userID, req.ImagePath, req.Language)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Resolve max tokens from active provider/model
	maxTokens := s.resolveMaxTokens(c)

	c.JSON(http.StatusCreated, dto.ConversationDTO{
		ID:         conv.ID,
		ImagePath:  conv.ImagePath,
		Title:      conv.Title,
		Summary:    conv.Summary,
		TokenCount: conv.TokenCount,
		MaxTokens:  maxTokens,
		Language:   conv.Language,
		CreatedAt:  conv.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt:  conv.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	})
}

// handleListConversations handles GET /api/chat/conversations
func (s *Server) handleListConversations(c *gin.Context) {
	userID := middleware.GetUserID(c)
	imagePath := c.Query("imagePath")

	var conversations []domain.Conversation
	var err error
	if imagePath != "" {
		conversations, err = s.conversationService.ListConversationsByImage(c.Request.Context(), userID, imagePath)
	} else {
		conversations, err = s.conversationService.ListConversations(c.Request.Context(), userID)
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	maxTokens := s.resolveMaxTokens(c)

	result := make([]dto.ConversationDTO, len(conversations))
	for i, conv := range conversations {
		result[i] = dto.ConversationDTO{
			ID:         conv.ID,
			ImagePath:  conv.ImagePath,
			Title:      conv.Title,
			Summary:    conv.Summary,
			TokenCount: conv.TokenCount,
			MaxTokens:  maxTokens,
			Language:   conv.Language,
			CreatedAt:  conv.CreatedAt.Format("2006-01-02T15:04:05Z"),
			UpdatedAt:  conv.UpdatedAt.Format("2006-01-02T15:04:05Z"),
		}
	}

	c.JSON(http.StatusOK, result)
}

// handleDeleteConversation handles DELETE /api/chat/conversations/:id
func (s *Server) handleDeleteConversation(c *gin.Context) {
	convID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgChatInvalidConversationID)
		return
	}

	userID := middleware.GetUserID(c)
	if err := s.conversationService.DeleteConversation(c.Request.Context(), uint(convID), userID); err != nil {
		s.respondError(c, http.StatusInternalServerError, i18n.MsgChatConversationNotFound)
		return
	}

	s.respondSuccess(c, http.StatusOK, i18n.MsgChatConversationDeleted)
}

// handleGetMessages handles GET /api/chat/conversations/:id/messages
func (s *Server) handleGetMessages(c *gin.Context) {
	convID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgChatInvalidConversationID)
		return
	}

	// Verify ownership
	userID := middleware.GetUserID(c)
	if _, err := s.conversationService.GetConversation(c.Request.Context(), uint(convID), userID); err != nil {
		s.respondError(c, http.StatusNotFound, i18n.MsgChatConversationNotFound)
		return
	}

	messages, err := s.conversationService.GetMessages(c.Request.Context(), uint(convID))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	result := make([]dto.ChatMessageDTO, len(messages))
	for i, msg := range messages {
		d := dto.ChatMessageDTO{
			ID:        msg.ID,
			Role:      msg.Role,
			Content:   msg.Content,
			CreatedAt: msg.CreatedAt.Format("2006-01-02T15:04:05Z"),
		}

		// Parse tool calls from JSON
		if msg.ToolCallsJSON != "" {
			var toolCalls []dto.ToolCallInfo
			if err := json.Unmarshal([]byte(msg.ToolCallsJSON), &toolCalls); err == nil {
				d.ToolCalls = toolCalls
			}
		}

		result[i] = d
	}

	c.JSON(http.StatusOK, result)
}

// handleSendMessage handles POST /api/chat/conversations/:id/messages with SSE streaming.
func (s *Server) handleSendMessage(c *gin.Context) {
	convID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgChatInvalidConversationID)
		return
	}

	var req dto.SendMessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		s.respondValidationError(c, http.StatusBadRequest, i18n.MsgChatContentRequired)
		return
	}

	// Verify ownership
	userID := middleware.GetUserID(c)
	if _, err := s.conversationService.GetConversation(c.Request.Context(), uint(convID), userID); err != nil {
		s.respondError(c, http.StatusNotFound, i18n.MsgChatConversationNotFound)
		return
	}

	// Create LLM chat client from active provider
	client, _, ok := s.llmFactory.CreateClient(c)
	if !ok {
		return // Error already written by CreateClient
	}

	chatClient, ok := llm.NewChatClient(client)
	if !ok {
		s.respondError(c, http.StatusInternalServerError, i18n.MsgChatLlmNoChatSupport)
		return
	}

	// Resolve max tokens from model cache
	maxTokens := s.resolveMaxTokens(c)

	// Set SSE headers
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Stream events
	c.Stream(func(w io.Writer) bool {
		eventHandler := func(event agent.ToolEvent) {
			data, _ := json.Marshal(dto.SSEEvent{
				Type:       event.Type,
				Name:       event.Name,
				Status:     event.Status,
				Result:     event.Result,
				Content:    event.Content,
				Error:      event.Error,
				TokenCount: event.TokenCount,
				MaxTokens:  event.MaxTokens,
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		_, err := s.agent.ProcessMessage(c.Request.Context(), uint(convID), req.Content, chatClient, eventHandler, maxTokens)
		if err != nil {
			data, _ := json.Marshal(dto.SSEEvent{
				Type:  "error",
				Error: err.Error(),
			})
			fmt.Fprintf(w, "data: %s\n\n", data)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}

		return false // Stop streaming
	})
}

// resolveMaxTokens resolves max tokens from active provider/model cache, falling back to config default.
func (s *Server) resolveMaxTokens(c *gin.Context) int {
	llmSettings := s.settingsLoader.LlmSettings()
	provider, ok := s.settingsLoader.LlmProvider(llmSettings.ActiveProvider)
	if !ok {
		return s.agentConfig.MaxConversationTokens
	}
	modelMax := s.conversationService.ResolveModelMaxTokens(provider.Alias, provider.Model)
	if modelMax > 0 {
		return modelMax
	}
	return s.agentConfig.MaxConversationTokens
}
