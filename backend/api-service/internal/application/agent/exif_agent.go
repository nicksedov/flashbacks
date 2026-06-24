package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/flashbacks/api-service/internal/infrastructure/llm"
)

// ExifAgent is a sub-agent that delegates EXIF metadata operations to the EXIF microservice via MCP.
type ExifAgent struct {
	serviceURL string
	backupDir  string
	httpClient *http.Client
}

// NewExifAgent creates a new EXIF agent that connects to the EXIF service MCP endpoint.
// backupDir is injected into write_gps calls so the EXIF service can back up originals.
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
	"read_exif", "read_gps", "read_all_metadata",
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

// ExecuteTool calls the EXIF service MCP endpoint to execute a tool.
// For write_gps, backup_dir is automatically injected into arguments if not already present.
func (ea *ExifAgent) ExecuteTool(ctx context.Context, name string, arguments json.RawMessage) (string, error) {
	url := fmt.Sprintf("%s/exif/mcp", ea.serviceURL)

	// Inject backup_dir for write_gps if not already provided
	if name == "write_gps" && ea.backupDir != "" {
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

	// Build MCP request
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

	// Parse MCP response
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

	if err := json.Unmarshal(respBody, &mcpResp); err != nil {
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
		"read_exif":         "Read all EXIF fields from image file (camera, lens, ISO, aperture, shutter, focal length, date, orientation)",
		"read_gps":          "Read GPS coordinates from image EXIF (latitude, longitude)",
		"read_all_metadata": "Read complete EXIF tag dump (all tags, raw values)",
		"write_gps":         "Write GPS coordinates to image EXIF (3-attempt strategy with backup)",
		"write_exif_field":  "Write arbitrary EXIF tag value (e.g., DateTimeOriginal, ImageDescription)",
		"strip_exif":        "Remove specified EXIF tags (or all if tags omitted)",
		"copy_exif":         "Copy EXIF data from source to target file",
		"compare_exif":      "Compare EXIF metadata between two images, return differences",
		"validate_exif":     "Validate EXIF integrity (check for corruption, InteropIFD issues)",
	}
	if desc, ok := descriptions[name]; ok {
		return desc
	}
	return name
}

func exifToolParams(name string) map[string]interface{} {
	switch name {
	case "read_exif", "read_gps", "read_all_metadata", "validate_exif":
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
				"path":  map[string]interface{}{"type": "string", "description": "Absolute path to the image file"},
				"tag":   map[string]interface{}{"type": "string", "description": "EXIF tag name"},
				"value": map[string]interface{}{"type": "string", "description": "Value to write"},
			},
			"required": []string{"path", "tag", "value"},
		}
	case "strip_exif":
		return map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{"type": "string", "description": "Absolute path to the image file"},
				"tags": map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}, "description": "Tags to remove (omit for all)"},
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
