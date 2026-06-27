package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/flashbacks/api-service/internal/infrastructure/llm"
)

// ExifAgent is a sub-agent that delegates EXIF metadata operations to the EXIF microservice via MCP.
// It manages an MCP session (initialize → Mcp-Session-Id) for all tool calls.
type ExifAgent struct {
	serviceURL string
	backupDir  string
	httpClient *http.Client

	sessionID string
	sessionMu sync.Mutex
}

// NewExifAgent creates a new EXIF agent that connects to the EXIF service MCP endpoint.
// backupDir is injected into all file-modifying tool calls (write_gps, write_exif_field,
// strip_exif, copy_exif) so the EXIF service can back up originals before modification.
func NewExifAgent(serviceURL, backupDir string) *ExifAgent {
	return &ExifAgent{
		serviceURL: serviceURL,
		backupDir:  backupDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// exifToolNames lists the MCP tools provided by the EXIF service.
var exifToolNames = []string{
	"read_exif_fields", "dump_exif_raw",
	"write_gps", "write_exif_field", "strip_exif", "copy_exif",
	"compare_exif", "validate_exif",
}

// ToolDefinitions returns the EXIF MCP tool definitions for use by the main agent.
func (ea *ExifAgent) ToolDefinitions() []llm.ToolDefinition {
	tools := make([]llm.ToolDefinition, 0, len(exifToolNames))

	for _, name := range exifToolNames {
		tools = append(tools, llm.ToolDefinition{
			Name:        name,
			Description: exifToolDescription(name),
			Parameters:  exifToolParams(name),
		})
	}

	return tools
}

// exifWriteTools is the set of tools that modify image files and need backup_dir injection.
var exifWriteTools = map[string]bool{
	"write_gps":        true,
	"write_exif_field": true,
	"strip_exif":       true,
	"copy_exif":        true,
}

// ExecuteTool calls the EXIF service MCP endpoint to execute a tool.
// Ensures an MCP session is initialized before sending tools/call.
// For file-modifying tools, backup_dir is automatically injected if not already provided.
func (ea *ExifAgent) ExecuteTool(ctx context.Context, name string, arguments json.RawMessage) (string, error) {
	// Inject backup_dir for file-modifying tools if not already provided
	if exifWriteTools[name] && ea.backupDir != "" {
		var args map[string]interface{}
		if err := json.Unmarshal(arguments, &args); err == nil {
			if _, hasBackupDir := args["backup_dir"]; !hasBackupDir {
				args["backup_dir"] = ea.backupDir
				if patched, err := json.Marshal(args); err == nil {
					arguments = patched
				}
			}
		}
	}

	// Ensure MCP session is initialized
	sessionID, err := ea.ensureSession(ctx)
	if err != nil {
		return "", fmt.Errorf("MCP session initialization failed: %w", err)
	}

	// Build MCP tools/call request
	url := fmt.Sprintf("%s/exif/mcp", ea.serviceURL)

	mcpReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "tools/call",
		"params": map[string]interface{}{
			"name":      name,
			"arguments": json.RawMessage(arguments),
		},
		"id": 1,
	}

	body, _ := json.Marshal(mcpReq)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create MCP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Mcp-Session-Id", sessionID)

	resp, err := ea.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("EXIF MCP request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read MCP response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("EXIF MCP returned %d: %s", resp.StatusCode, string(respBody))
	}

	// Parse MCP response, handling both application/json and text/event-stream.
	jsonBody := extractJSONFromMCPResponse(resp, respBody)

	var mcpResp struct {
		Result struct {
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error *struct {
			Message string `json:"message"`
		} `json:"error"`
	}

	if err := json.Unmarshal(jsonBody, &mcpResp); err != nil {
		return string(respBody), nil
	}

	if mcpResp.Error != nil {
		return "", fmt.Errorf("EXIF MCP error: %s", mcpResp.Error.Message)
	}

	// Extract text content
	var result string
	for _, c := range mcpResp.Result.Content {
		if c.Type == "text" {
			result += c.Text
		}
	}

	return result, nil
}

// ensureSession returns the current session ID, initializing a new MCP session if needed.
// Thread-safe via sessionMu. Performs the full MCP handshake: initialize → notifications/initialized.
func (ea *ExifAgent) ensureSession(ctx context.Context) (string, error) {
	ea.sessionMu.Lock()
	defer ea.sessionMu.Unlock()

	if ea.sessionID != "" {
		return ea.sessionID, nil
	}

	url := fmt.Sprintf("%s/exif/mcp", ea.serviceURL)

	// Step 1: Send initialize request
	initReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "initialize",
		"params": map[string]interface{}{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]interface{}{},
			"clientInfo": map[string]interface{}{
				"name":    "flashbacks-api-service",
				"version": "1.0.0",
			},
		},
		"id": 0,
	}

	body, _ := json.Marshal(initReq)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create initialize request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")

	resp, err := ea.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("initialize request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("initialize returned %d: %s", resp.StatusCode, string(respBody))
	}

	sessionID := resp.Header.Get("Mcp-Session-Id")
	if sessionID == "" {
		return "", fmt.Errorf("initialize response missing Mcp-Session-Id header")
	}

	// Step 2: Send initialized notification to complete the handshake
	notifReq := map[string]interface{}{
		"jsonrpc": "2.0",
		"method":  "notifications/initialized",
	}

	notifBody, _ := json.Marshal(notifReq)
	notifHTTPReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(notifBody))
	if err != nil {
		return "", fmt.Errorf("failed to create initialized notification: %w", err)
	}
	notifHTTPReq.Header.Set("Content-Type", "application/json")
	notifHTTPReq.Header.Set("Accept", "application/json, text/event-stream")
	notifHTTPReq.Header.Set("Mcp-Session-Id", sessionID)

	notifResp, err := ea.httpClient.Do(notifHTTPReq)
	if err != nil {
		return "", fmt.Errorf("initialized notification failed: %w", err)
	}
	notifResp.Body.Close()

	if notifResp.StatusCode != http.StatusOK && notifResp.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("initialized notification returned %d", notifResp.StatusCode)
	}

	ea.sessionID = sessionID
	return sessionID, nil
}

// extractJSONFromMCPResponse extracts the JSON payload from an MCP Streamable HTTP response.
// For application/json responses, the body is used as-is.
// For text/event-stream (SSE) responses, the JSON is extracted from the last "data:" line.
func extractJSONFromMCPResponse(resp *http.Response, body []byte) []byte {
	contentType := resp.Header.Get("Content-Type")
	if strings.HasPrefix(contentType, "text/event-stream") {
		// SSE format: extract JSON from "data: {...}" lines
		lines := strings.Split(string(body), "\n")
		for i := len(lines) - 1; i >= 0; i-- {
			line := strings.TrimSpace(lines[i])
			if strings.HasPrefix(line, "data:") {
				return []byte(strings.TrimSpace(strings.TrimPrefix(line, "data:")))
			}
		}
		return body
	}
	// Default: assume application/json
	return body
}

// IsExifTool returns true if the tool name belongs to the EXIF service.
func IsExifTool(name string) bool {
	for _, t := range exifToolNames {
		if t == name {
			return true
		}
	}
	return false
}

func exifToolDescription(name string) string {
	descriptions := map[string]string{
		"read_exif_fields": "Read structured EXIF fields from image file (camera, lens, ISO, aperture, shutter, focal length, date, orientation, GPS). Reads directly from the file — always current.",
		"dump_exif_raw":    "Dump all raw EXIF tags from image file (complete tag listing, unprocessed values)",
		"write_gps":        "Write GPS coordinates to image EXIF (3-attempt strategy with backup)",
		"write_exif_field": "Write arbitrary EXIF tag value (e.g., DateTimeOriginal, ImageDescription)",
		"strip_exif":       "Remove specified EXIF tags (or all if tags omitted)",
		"copy_exif":        "Copy EXIF data from source to target file",
		"compare_exif":     "Compare EXIF metadata between two images, return differences",
		"validate_exif":    "Validate EXIF integrity (check for corruption, InteropIFD issues)",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return name
}

func exifToolParams(name string) map[string]interface{} {
	switch name {
	case "read_exif_fields", "dump_exif_raw", "validate_exif":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "Absolute path to the image file",
				},
			},
			"required": []string{"path"},
		}
	case "write_gps":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":       map[string]interface{}{"type": "string", "description": "Absolute path to the image file"},
				"latitude":   map[string]interface{}{"type": "number", "description": "GPS latitude (-90 to 90)"},
				"longitude":  map[string]interface{}{"type": "number", "description": "GPS longitude (-180 to 180)"},
				"backup_dir": map[string]interface{}{"type": "string", "description": "Directory where a backup copy of the original file will be stored before modification"},
			},
			"required": []string{"path", "latitude", "longitude", "backup_dir"},
		}
	case "write_exif_field":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":       map[string]interface{}{"type": "string", "description": "Absolute path to the image file"},
				"tag":        map[string]interface{}{"type": "string", "description": "EXIF tag name"},
				"value":      map[string]interface{}{"type": "string", "description": "Value to write"},
				"backup_dir": map[string]interface{}{"type": "string", "description": "Directory for backup copies (auto-injected if not provided)"},
			},
			"required": []string{"path", "tag", "value"},
		}
	case "strip_exif":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path":       map[string]interface{}{"type": "string", "description": "Absolute path to the image file"},
				"tags":       map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Tags to remove (omit for all)"},
				"backup_dir": map[string]interface{}{"type": "string", "description": "Directory for backup copies (auto-injected if not provided)"},
			},
			"required": []string{"path"},
		}
	case "copy_exif":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"source_path": map[string]interface{}{"type": "string", "description": "Source file path"},
				"target_path": map[string]interface{}{"type": "string", "description": "Target file path"},
				"tags":        map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Tags to copy (omit for all)"},
				"backup_dir":  map[string]interface{}{"type": "string", "description": "Directory for backup copies (auto-injected if not provided)"},
			},
			"required": []string{"source_path", "target_path"},
		}
	case "compare_exif":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path1": map[string]interface{}{"type": "string", "description": "Path to first image"},
				"path2": map[string]interface{}{"type": "string", "description": "Path to second image"},
			},
			"required": []string{"path1", "path2"},
		}
	default:
		return map[string]interface{}{"type": "object", "properties": map[string]interface{}{}}
	}
}
