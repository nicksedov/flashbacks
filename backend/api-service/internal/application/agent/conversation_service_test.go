package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/flashbacks/api-service/internal/domain"
	"github.com/flashbacks/api-service/internal/infrastructure/llm"
	"github.com/flashbacks/api-service/internal/testutil"
)

func TestCreateConversation(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)

	conv, err := svc.CreateConversation(context.Background(), 1, "/photos/test.jpg", "en")
	if err != nil {
		t.Fatalf("CreateConversation failed: %v", err)
	}
	if conv.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if conv.UserID != 1 {
		t.Errorf("expected UserID=1, got %d", conv.UserID)
	}
	if conv.ImagePath != "/photos/test.jpg" {
		t.Errorf("expected ImagePath='/photos/test.jpg', got %q", conv.ImagePath)
	}
	if conv.Title != "New Chat" {
		t.Errorf("expected Title='New Chat', got %q", conv.Title)
	}
	if conv.Language != "en" {
		t.Errorf("expected Language='en', got %q", conv.Language)
	}
}

func TestGetConversation(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "/img.jpg", "en")

	// Correct user
	got, err := svc.GetConversation(context.Background(), conv.ID, 1)
	if err != nil {
		t.Fatalf("GetConversation failed: %v", err)
	}
	if got.ID != conv.ID {
		t.Errorf("expected ID=%d, got %d", conv.ID, got.ID)
	}

	// Wrong user
	_, err = svc.GetConversation(context.Background(), conv.ID, 999)
	if err == nil {
		t.Fatal("expected error for wrong user, got nil")
	}
}

func TestGetConversationByID(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "/img.jpg", "en")

	got, err := svc.GetConversationByID(context.Background(), conv.ID)
	if err != nil {
		t.Fatalf("GetConversationByID failed: %v", err)
	}
	if got.ID != conv.ID {
		t.Errorf("expected ID=%d, got %d", conv.ID, got.ID)
	}

	// Non-existent
	_, err = svc.GetConversationByID(context.Background(), 9999)
	if err == nil {
		t.Fatal("expected error for non-existent ID")
	}
}

func TestListConversations(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv1, _ := svc.CreateConversation(context.Background(), 1, "/img1.jpg", "en")
	conv2, _ := svc.CreateConversation(context.Background(), 1, "/img2.jpg", "ru")
	conv3, _ := svc.CreateConversation(context.Background(), 2, "/other.jpg", "en")

	// Add messages so conversations are non-empty
	svc.AddMessage(context.Background(), conv1.ID, domain.ConversationMessage{Role: "user", Content: "hello"})
	svc.AddMessage(context.Background(), conv2.ID, domain.ConversationMessage{Role: "user", Content: "hello"})
	svc.AddMessage(context.Background(), conv3.ID, domain.ConversationMessage{Role: "user", Content: "hello"})

	list, err := svc.ListConversations(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 conversations for user 1, got %d", len(list))
	}

	list2, _ := svc.ListConversations(context.Background(), 2)
	if len(list2) != 1 {
		t.Errorf("expected 1 conversation for user 2, got %d", len(list2))
	}
}

func TestListConversations_EmptyFiltered(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv1, _ := svc.CreateConversation(context.Background(), 1, "/img1.jpg", "en")
	conv2, _ := svc.CreateConversation(context.Background(), 1, "/img2.jpg", "en")

	// Only add message to conv1, leave conv2 empty
	svc.AddMessage(context.Background(), conv1.ID, domain.ConversationMessage{Role: "user", Content: "hello"})

	list, err := svc.ListConversations(context.Background(), 1)
	if err != nil {
		t.Fatalf("ListConversations failed: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("expected 1 non-empty conversation, got %d", len(list))
	}
	if len(list) > 0 && list[0].ID != conv1.ID {
		t.Errorf("expected conv1 to be listed, got conv %d", list[0].ID)
	}
	_ = conv2 // conv2 should not appear in the list
}

func TestDeleteConversation(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "/img.jpg", "en")

	// Add some messages
	svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{Role: "user", Content: "hello"})
	svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{Role: "assistant", Content: "hi there"})

	// Wrong user
	err := svc.DeleteConversation(context.Background(), conv.ID, 999)
	if err == nil {
		t.Fatal("expected error deleting with wrong user")
	}

	// Correct user
	err = svc.DeleteConversation(context.Background(), conv.ID, 1)
	if err != nil {
		t.Fatalf("DeleteConversation failed: %v", err)
	}

	// Verify deleted
	_, err = svc.GetConversationByID(context.Background(), conv.ID)
	if err == nil {
		t.Fatal("expected error after deletion")
	}

	// Verify messages deleted
	msgs, _ := svc.GetMessages(context.Background(), conv.ID)
	if len(msgs) != 0 {
		t.Errorf("expected 0 messages after deletion, got %d", len(msgs))
	}
}

func TestAddAndGetMessages(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "", "en")

	err := svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{Role: "user", Content: "Hello"})
	if err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}
	err = svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{Role: "assistant", Content: "Hi!"})
	if err != nil {
		t.Fatalf("AddMessage failed: %v", err)
	}

	msgs, err := svc.GetMessages(context.Background(), conv.ID)
	if err != nil {
		t.Fatalf("GetMessages failed: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "Hello" {
		t.Errorf("unexpected first message: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "Hi!" {
		t.Errorf("unexpected second message: %+v", msgs[1])
	}
}

func TestAutoTitleFromFirstUserMessage(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "", "en")

	svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{Role: "user", Content: "What is in this photo?"})

	got, _ := svc.GetConversationByID(context.Background(), conv.ID)
	if got.Title != "What is in this photo?" {
		t.Errorf("expected auto-title, got %q", got.Title)
	}
}

func TestAutoTitleTruncated(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "", "en")

	longMsg := "This is a very long message that exceeds fifty characters and should be truncated"
	svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{Role: "user", Content: longMsg})

	got, _ := svc.GetConversationByID(context.Background(), conv.ID)
	if len(got.Title) > 54 { // 50 + "..."
		t.Errorf("title too long: %q (len=%d)", got.Title, len(got.Title))
	}
}

func TestAutoTitleCyrillicUTF8(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "", "ru")

	// 60 Cyrillic characters = 120 bytes; the old byte-slice at [:50] would
	// split a 2-byte character and produce invalid UTF-8 like 0xd0 0x2e.
	longCyrillic := "Описать изображение на этой фотографии в высоком разрешении и деталях"
	svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{Role: "user", Content: longCyrillic})

	got, _ := svc.GetConversationByID(context.Background(), conv.ID)

	// Title must be valid UTF-8
	if !utf8.ValidString(got.Title) {
		t.Errorf("title contains invalid UTF-8: %q", got.Title)
	}

	// Title must be truncated (50 runes + "...")
	if len([]rune(got.Title)) > 54 {
		t.Errorf("title too long: %q (runes=%d)", got.Title, len([]rune(got.Title)))
	}

	// Title must end with "..."
	if !strings.HasSuffix(got.Title, "...") {
		t.Errorf("title should end with '...': %q", got.Title)
	}

	// Title must be a valid prefix of the original (by runes)
	origRunes := []rune(longCyrillic)
	titleRunes := []rune(strings.TrimSuffix(got.Title, "..."))
	for i := 0; i < len(titleRunes); i++ {
		if titleRunes[i] != origRunes[i] {
			t.Errorf("title rune %d mismatch: got %q, expected %q", i, titleRunes[i], origRunes[i])
		}
	}
}

func TestCountTokens(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "", "en")

	// Add known-length messages
	svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{Role: "user", Content: "hello world"})         // ~3 tokens
	svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{Role: "assistant", Content: "how can I help"}) // ~4 tokens

	count, err := svc.CountTokens(context.Background(), conv.ID)
	if err != nil {
		t.Fatalf("CountTokens failed: %v", err)
	}
	if count < 2 || count > 20 {
		t.Errorf("unexpected token count: %d", count)
	}
}

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		name   string
		text   string
		minExp int
		maxExp int
	}{
		{"empty", "", 0, 0},
		{"short english", "hello", 1, 3},
		{"sentence", "The quick brown fox jumps over the lazy dog", 8, 14},
		{"russian", "Привет мир", 4, 7}, // ~10 chars, ~2 chars/token = 5
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := estimateTokens(tt.text)
			if got < tt.minExp || got > tt.maxExp {
				t.Errorf("estimateTokens(%q) = %d, want [%d, %d]", tt.text, got, tt.minExp, tt.maxExp)
			}
		})
	}
}

func TestMessagesToChatMessages(t *testing.T) {
	messages := []domain.ConversationMessage{
		{Role: "system", Content: "You are helpful"},
		{Role: "user", Content: "Hello"},
		{
			Role:          "assistant",
			Content:       "",
			ToolCallsJSON: `[{"id":"call_1","name":"describe_image","arguments":{"image_path":"/test.jpg"}}]`,
		},
		{Role: "tool", Content: "A beautiful sunset", ToolCallID: "call_1"},
	}

	result := MessagesToChatMessages(messages)

	if len(result) != 4 {
		t.Fatalf("expected 4 messages, got %d", len(result))
	}

	// Check tool calls restored
	if len(result[2].ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result[2].ToolCalls))
	}
	if result[2].ToolCalls[0].Name != "describe_image" {
		t.Errorf("expected tool name 'describe_image', got %q", result[2].ToolCalls[0].Name)
	}

	// Check tool result
	if result[3].ToolCallID != "call_1" {
		t.Errorf("expected ToolCallID='call_1', got %q", result[3].ToolCallID)
	}
}

func TestMessagesToChatMessages_InvalidJSON(t *testing.T) {
	messages := []domain.ConversationMessage{
		{
			Role:          "assistant",
			ToolCallsJSON: "invalid json",
		},
	}

	result := MessagesToChatMessages(messages)
	if len(result) != 1 {
		t.Fatalf("expected 1 message, got %d", len(result))
	}
	// Invalid JSON should be silently ignored
	if len(result[0].ToolCalls) != 0 {
		t.Errorf("expected no tool calls for invalid JSON, got %d", len(result[0].ToolCalls))
	}
}

// mockChatClient implements llm.ChatClient for testing.
type mockChatClient struct {
	responses []*llm.ChatResponse
	callIndex int
}

func (m *mockChatClient) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	if m.callIndex >= len(m.responses) {
		// Default: end turn with empty message
		return &llm.ChatResponse{
			Message:    llm.ChatMessage{Role: "assistant", Content: "done"},
			StopReason: "end_turn",
		}, nil
	}
	resp := m.responses[m.callIndex]
	m.callIndex++
	return resp, nil
}

func (m *mockChatClient) Recognize(ctx context.Context, imagePath, systemPrompt, userMessage string) (string, error) {
	return "", nil
}

func (m *mockChatClient) ListModels(ctx context.Context) ([]llm.ModelInfo, error) {
	return nil, nil
}

func TestSummarizeOlderMessages(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "", "en")

	// Add 8 messages
	for i := 0; i < 8; i++ {
		svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{
			Role:    "user",
			Content: "message " + string(rune('A'+i)),
		})
	}

	mock := &mockChatClient{
		responses: []*llm.ChatResponse{
			{
				Message:    llm.ChatMessage{Role: "assistant", Content: "Summary of earlier messages"},
				StopReason: "end_turn",
			},
		},
	}

	// Keep last 4, summarize the first 4
	err := svc.SummarizeOlderMessages(context.Background(), conv.ID, 4, mock)
	if err != nil {
		t.Fatalf("SummarizeOlderMessages failed: %v", err)
	}

	msgs, _ := svc.GetMessages(context.Background(), conv.ID)
	// Should have: 1 summary + 4 kept = 5
	if len(msgs) != 5 {
		t.Errorf("expected 5 messages after summarization, got %d", len(msgs))
	}

	// First message should be the summary
	if msgs[0].Role != "system" {
		t.Errorf("expected first message to be system summary, got role=%q", msgs[0].Role)
	}
}

func TestSummarizeOlderMessages_NothingToSummarize(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "", "en")

	svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{Role: "user", Content: "only one"})

	mock := &mockChatClient{}
	err := svc.SummarizeOlderMessages(context.Background(), conv.ID, 6, mock)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should still have 1 message
	msgs, _ := svc.GetMessages(context.Background(), conv.ID)
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
}

func TestListConversationsByImage(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv1, _ := svc.CreateConversation(context.Background(), 1, "/img1.jpg", "en")
	conv2, _ := svc.CreateConversation(context.Background(), 1, "/img1.jpg", "en")
	conv3, _ := svc.CreateConversation(context.Background(), 1, "/img2.jpg", "en")
	conv4, _ := svc.CreateConversation(context.Background(), 2, "/img1.jpg", "en")

	// Add messages to make them non-empty
	svc.AddMessage(context.Background(), conv1.ID, domain.ConversationMessage{Role: "user", Content: "hello"})
	svc.AddMessage(context.Background(), conv2.ID, domain.ConversationMessage{Role: "user", Content: "hello"})
	svc.AddMessage(context.Background(), conv3.ID, domain.ConversationMessage{Role: "user", Content: "hello"})
	svc.AddMessage(context.Background(), conv4.ID, domain.ConversationMessage{Role: "user", Content: "hello"})

	list, err := svc.ListConversationsByImage(context.Background(), 1, "/img1.jpg")
	if err != nil {
		t.Fatalf("ListConversationsByImage failed: %v", err)
	}
	if len(list) != 2 {
		t.Errorf("expected 2 conversations, got %d", len(list))
	}

	list2, _ := svc.ListConversationsByImage(context.Background(), 2, "/img1.jpg")
	if len(list2) != 1 {
		t.Errorf("expected 1 conversation for user 2, got %d", len(list2))
	}

	list3, _ := svc.ListConversationsByImage(context.Background(), 1, "/nonexistent.jpg")
	if len(list3) != 0 {
		t.Errorf("expected 0 conversations, got %d", len(list3))
	}
}

func TestAddMessage_TokenIncrement(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "/img.jpg", "en")

	// Initial token count should be 0
	got, _ := svc.GetConversationByID(context.Background(), conv.ID)
	if got.TokenCount != 0 {
		t.Errorf("expected initial TokenCount=0, got %d", got.TokenCount)
	}

	// Add a message with known token count
	svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{
		Role:       "user",
		Content:    "hello world",
		TokenCount: 5,
	})

	got, _ = svc.GetConversationByID(context.Background(), conv.ID)
	if got.TokenCount != 5 {
		t.Errorf("expected TokenCount=5 after first message, got %d", got.TokenCount)
	}

	// Add another message
	svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{
		Role:       "assistant",
		Content:    "hi there, how can I help you today?",
		TokenCount: 10,
	})

	got, _ = svc.GetConversationByID(context.Background(), conv.ID)
	if got.TokenCount != 15 {
		t.Errorf("expected TokenCount=15 after second message, got %d", got.TokenCount)
	}
}

func TestResolveModelMaxTokens(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)

	// Insert a model cache entry with context length
	modelsJSON := `[{"id":"llama3:latest","name":"llama3:latest","size":4000000000,"contextLength":8192},{"id":"mistral:latest","name":"mistral:latest","size":3000000000,"contextLength":32768}]`
	cache := domain.LlmProviderModelCache{
		ProviderAlias: "ollama_1",
		ModelsJSON:    modelsJSON,
	}
	db.Create(&cache)

	// Found model
	maxTokens := svc.ResolveModelMaxTokens("ollama_1", "llama3:latest")
	if maxTokens != 8192 {
		t.Errorf("expected 8192, got %d", maxTokens)
	}

	maxTokens2 := svc.ResolveModelMaxTokens("ollama_1", "mistral:latest")
	if maxTokens2 != 32768 {
		t.Errorf("expected 32768, got %d", maxTokens2)
	}

	// Unknown model
	maxTokens3 := svc.ResolveModelMaxTokens("ollama_1", "unknown:model")
	if maxTokens3 != 0 {
		t.Errorf("expected 0 for unknown model, got %d", maxTokens3)
	}

	// Unknown provider
	maxTokens4 := svc.ResolveModelMaxTokens("nonexistent", "llama3:latest")
	if maxTokens4 != 0 {
		t.Errorf("expected 0 for unknown provider, got %d", maxTokens4)
	}
}

func TestGenerateDisplaySummary_FallbackToFirstMessage(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "/img.jpg", "en")

	// Add only 1 user message (fewer than 2)
	svc.AddMessage(context.Background(), conv.ID, domain.ConversationMessage{
		Role:       "user",
		Content:    "What is in this photo?",
		TokenCount: 5,
	})

	mock := &mockChatClient{}
	svc.GenerateDisplaySummary(context.Background(), conv.ID, mock)

	got, _ := svc.GetConversationByID(context.Background(), conv.ID)
	if got.Summary != "What is in this photo?" {
		t.Errorf("expected fallback summary, got %q", got.Summary)
	}
}

func TestGenerateDisplaySummary_SkipsIfAlreadyHasSummary(t *testing.T) {
	db, cleanup := testutil.NewTestDB(t)
	defer cleanup()

	svc := NewConversationService(db)
	conv, _ := svc.CreateConversation(context.Background(), 1, "/img.jpg", "en")

	// Set existing summary
	db.Model(&domain.Conversation{}).Where("id = ?", conv.ID).Update("summary", "Existing summary")

	mock := &mockChatClient{}
	svc.GenerateDisplaySummary(context.Background(), conv.ID, mock)

	got, _ := svc.GetConversationByID(context.Background(), conv.ID)
	if got.Summary != "Existing summary" {
		t.Errorf("expected existing summary to be preserved, got %q", got.Summary)
	}
}

// Ensure json.RawMessage tool call arguments survive the round-trip
func TestToolCallsJSON_RoundTrip(t *testing.T) {
	args := json.RawMessage(`{"image_path":"/test.jpg","language":"en"}`)
	toolCalls := []llm.ToolCall{
		{ID: "call_123", Name: "describe_image", Arguments: args},
	}

	data, _ := json.Marshal(toolCalls)
	var restored []llm.ToolCall
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if len(restored) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(restored))
	}
	if restored[0].Name != "describe_image" {
		t.Errorf("expected name 'describe_image', got %q", restored[0].Name)
	}
	if string(restored[0].Arguments) != string(args) {
		t.Errorf("arguments mismatch: %s vs %s", restored[0].Arguments, args)
	}
}
