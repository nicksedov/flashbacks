package imaging

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"

	"github.com/flashbacks/api-service/internal/infrastructure/llm/prompts"
)

//go:embed prompts/ocr_system.txt
var ocrPromptFS embed.FS

// ocrPromptData holds template data for the OCR system prompt.
type ocrPromptData struct {
	Language string
}

// ocrSystemTemplate is parsed once at init time.
var ocrSystemTemplate *template.Template

func init() {
	content, err := ocrPromptFS.ReadFile("prompts/ocr_system.txt")
	if err != nil {
		panic(fmt.Sprintf("failed to read embedded OCR prompt: %v", err))
	}
	ocrSystemTemplate, err = template.New("ocr_system.txt").Parse(string(content))
	if err != nil {
		panic(fmt.Sprintf("failed to parse OCR prompt template: %v", err))
	}
}

// buildOcrSystemPrompt creates the system prompt for VL LLM.
func buildOcrSystemPrompt(language string) string {
	langName := prompts.LanguageCodeToName(language)

	var buf bytes.Buffer
	if err := ocrSystemTemplate.Execute(&buf, ocrPromptData{Language: langName}); err != nil {
		panic(fmt.Sprintf("failed to execute OCR prompt template: %v", err))
	}
	return buf.String()
}
