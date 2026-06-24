package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
	"unicode"
)

//go:embed prompts/*.txt
var promptsFS embed.FS

// promptTemplates holds parsed templates for prompts with placeholders.
var promptTemplates = make(map[string]*template.Template)

func init() {
	templatedPrompts := []string{
		"prompts/action_describe.txt",
		"prompts/action_tags.txt",
		"prompts/action_recognize_text.txt",
		"prompts/action_ask_question.txt",
		"prompts/action_default.txt",
	}

	for _, name := range templatedPrompts {
		content, err := promptsFS.ReadFile(name)
		if err != nil {
			panic(fmt.Sprintf("failed to read embedded prompt %s: %v", name, err))
		}
		tmpl, err := template.New(name).Parse(string(content))
		if err != nil {
			panic(fmt.Sprintf("failed to parse prompt template %s: %v", name, err))
		}
		promptTemplates[name] = tmpl
	}
}

// RenderPrompt renders a templated prompt with the given data.
func RenderPrompt(name string, data any) string {
	tmpl, ok := promptTemplates[name]
	if !ok {
		panic(fmt.Sprintf("prompt template not found: %s", name))
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		panic(fmt.Sprintf("failed to execute prompt template %s: %v", name, err))
	}
	return buf.String()
}

// LoadPrompt reads a static prompt file (no template rendering).
func LoadPrompt(name string) string {
	content, err := promptsFS.ReadFile(name)
	if err != nil {
		panic(fmt.Sprintf("failed to read embedded prompt %s: %v", name, err))
	}
	return string(content)
}

// LanguageCodeToName converts a language code (e.g. "en", "ru") to a full language name.
func LanguageCodeToName(code string) string {
	switch code {
	case "ru":
		return "Russian"
	default:
		return "English"
	}
}

// DetectQuestionLanguage determines the language of a user question based on character analysis.
func DetectQuestionLanguage(text string) string {
	cyrillicCount := 0
	latinCount := 0

	for _, ch := range text {
		if unicode.Is(unicode.Cyrillic, ch) {
			cyrillicCount++
		} else if unicode.Is(unicode.Latin, ch) {
			latinCount++
		}
	}

	if cyrillicCount > latinCount {
		return "Russian"
	}
	return "English"
}

// ActionPromptData holds template data for action prompts.
type ActionPromptData struct {
	ResponseLanguage string
}

// RecognizeTextPromptData holds template data for OCR prompts.
type RecognizeTextPromptData struct {
	NoTextMessage string
}

// AskQuestionPromptData holds template data for question prompts.
type AskQuestionPromptData struct {
	Question         string
	QuestionLanguage string
}

// BuildActionPrompt creates the system prompt for an AI action.
func BuildActionPrompt(action, question, language string) string {
	responseLang := LanguageCodeToName(language)

	switch action {
	case "describe":
		return RenderPrompt("prompts/action_describe.txt", ActionPromptData{ResponseLanguage: responseLang})
	case "tags":
		return LoadPrompt("prompts/action_tags.txt")
	case "recognizeText":
		noTextMsg := "No text detected."
		if language == "ru" {
			noTextMsg = "Текст не обнаружен."
		}
		return RenderPrompt("prompts/action_recognize_text.txt", RecognizeTextPromptData{NoTextMessage: noTextMsg})
	case "askQuestion":
		questionLang := DetectQuestionLanguage(question)
		return RenderPrompt("prompts/action_ask_question.txt", AskQuestionPromptData{
			Question:         question,
			QuestionLanguage: questionLang,
		})
	default:
		return LoadPrompt("prompts/action_default.txt")
	}
}

// BuildActionUserMessage returns the user message for the LLM based on action type.
func BuildActionUserMessage(action string) string {
	switch action {
	case "describe":
		return "Describe this image in detail."
	case "tags":
		return "Generate tags for this image."
	case "recognizeText":
		return "Recognize and extract all text from this image."
	case "askQuestion":
		return "Answer the question about this image."
	default:
		return "Analyze this image."
	}
}

// ParseTags parses a comma-separated or newline-separated list of tags.
func ParseTags(input string) []string {
	parts := strings.Split(input, ",")
	if len(parts) == 1 {
		parts = strings.Split(input, "\n")
	}

	var tags []string
	for _, part := range parts {
		tag := strings.TrimSpace(part)
		tag = strings.Trim(tag, `"'`)
		if tag != "" {
			tags = append(tags, tag)
		}
	}
	return tags
}
