package anthropic

import (
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"
)

// TestImageMediaTypes tests that all common image media types are properly handled.
func TestImageMediaTypes(t *testing.T) {
	// Test data: a minimal 1x1 PNG image (base64 encoded)
	minimalPNG := base64.StdEncoding.EncodeToString([]byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, // PNG signature
		0x00, 0x00, 0x00, 0x0D, // IHDR length
		0x49, 0x48, 0x44, 0x52, // IHDR type
		0x00, 0x00, 0x00, 0x01, // width: 1
		0x00, 0x00, 0x00, 0x01, // height: 1
		0x08, 0x02, 0x00, 0x00, 0x00, // bit depth, color type, etc.
	})

	tests := []struct {
		name      string
		mediaType string
		data      string
	}{
		{
			name:      "PNG image",
			mediaType: "image/png",
			data:      minimalPNG,
		},
		{
			name:      "JPEG image",
			mediaType: "image/jpeg",
			data:      "/9j/4AAQSkZJRgABAQEAYABgAAD/2wBD", // Minimal JPEG header
		},
		{
			name:      "WebP image",
			mediaType: "image/webp",
			data:      "UklGRiQAAABXRUJQVlA4IBgAAAAwAQCdASoBAAEAAQA", // Minimal WebP
		},
		{
			name:      "GIF image",
			mediaType: "image/gif",
			data:      "R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7", // Minimal GIF
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imageBlock := ContentBlock{
				Type: "image",
				Source: &ImageSource{
					Type:      "base64",
					MediaType: tt.mediaType,
					Data:      tt.data,
				},
			}

			// Test marshaling
			data, err := json.Marshal(imageBlock)
			if err != nil {
				t.Fatalf("failed to marshal image block: %v", err)
			}

			var unmarshaled ContentBlock
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				t.Fatalf("failed to unmarshal image block: %v", err)
			}

			// Verify the type is preserved
			if unmarshaled.Type != "image" {
				t.Errorf("expected type 'image', got %s", unmarshaled.Type)
			}

			// Verify the source is not nil
			if unmarshaled.Source == nil {
				t.Fatal("expected source to be non-nil")
			}

			// Verify media type is preserved
			if unmarshaled.Source.MediaType != tt.mediaType {
				t.Errorf("expected media type %s, got %s", tt.mediaType, unmarshaled.Source.MediaType)
			}

			// Verify data is preserved
			if unmarshaled.Source.Data != tt.data {
				t.Errorf("expected data to be preserved")
			}
		})
	}
}

// TestImageWithTextContent tests that images can be combined with text in the same message.
func TestImageWithTextContent(t *testing.T) {
	content := MessageContent{
		{Type: "text", Text: "What's in this image?"},
		{
			Type: "image",
			Source: &ImageSource{
				Type:      "base64",
				MediaType: "image/png",
				Data:      "base64datahere",
			},
		},
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("failed to marshal content: %v", err)
	}

	result := string(data)

	// Verify it's an array (mixed content)
	if result[0] != '[' {
		t.Errorf("expected array format for mixed content, got: %s", result)
	}

	// Verify both text and image are present
	if !strings.Contains(result, `"type":"text"`) && !strings.Contains(result, `"type": "text"`) {
		t.Error("expected text block in output")
	}
	if !strings.Contains(result, `"type":"image"`) && !strings.Contains(result, `"type": "image"`) {
		t.Error("expected image block in output")
	}

	// Verify image source fields
	if !strings.Contains(result, `"media_type":`) && !strings.Contains(result, `"media_type":`) {
		t.Error("expected media_type in image source")
	}
}

// TestImageInMessage tests that images work in full message structures.
func TestImageInMessage(t *testing.T) {
	req := &Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages: []Message{
			{
				Role: RoleUser,
				Content: MessageContent{
					{Type: "text", Text: "Describe this image"},
					{
						Type: "image",
						Source: &ImageSource{
							Type:      "base64",
							MediaType: "image/jpeg",
							Data:      "fakebase64data",
						},
					},
				},
			},
		},
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var unmarshaled Request
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	// Verify message structure
	if len(unmarshaled.Messages) != 1 {
		t.Fatalf("expected 1 message, got %d", len(unmarshaled.Messages))
	}

	msg := unmarshaled.Messages[0]
	if len(msg.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(msg.Content))
	}

	// Verify text block
	if msg.Content[0].Type != "text" {
		t.Errorf("expected first block to be text, got %s", msg.Content[0].Type)
	}

	// Verify image block
	if msg.Content[1].Type != "image" {
		t.Errorf("expected second block to be image, got %s", msg.Content[1].Type)
	}

	if msg.Content[1].Source == nil {
		t.Fatal("expected image source to be non-nil")
	}

	if msg.Content[1].Source.MediaType != "image/jpeg" {
		t.Errorf("expected media type image/jpeg, got %s", msg.Content[1].Source.MediaType)
	}
}

// TestImageSourceValidation tests validation of image source fields.
func TestImageSourceValidation(t *testing.T) {
	tests := []struct {
		name        string
		source      *ImageSource
		expectError bool
	}{
		{
			name: "valid base64 image",
			source: &ImageSource{
				Type:      "base64",
				MediaType: "image/png",
				Data:      "validbase64data",
			},
			expectError: false,
		},
		{
			name: "missing type field",
			source: &ImageSource{
				MediaType: "image/png",
				Data:      "data",
			},
			expectError: false, // JSON unmarshaling will set empty string
		},
		{
			name: "missing media type",
			source: &ImageSource{
				Type: "base64",
				Data: "data",
			},
			expectError: false, // JSON unmarshaling will set empty string
		},
		{
			name:        "nil source",
			source:      nil,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			imageBlock := ContentBlock{
				Type:   "image",
				Source: tt.source,
			}

			// Marshal and unmarshal to test JSON handling
			data, err := json.Marshal(imageBlock)
			if err != nil {
				if !tt.expectError {
					t.Errorf("unexpected error: %v", err)
				}
				return
			}

			var unmarshaled ContentBlock
			if err := json.Unmarshal(data, &unmarshaled); err != nil {
				if !tt.expectError {
					t.Errorf("unexpected unmarshal error: %v", err)
				}
				return
			}

			if tt.expectError {
				t.Error("expected error but got none")
			}
		})
	}
}

// TestMultipleImages tests handling multiple images in a single message.
func TestMultipleImages(t *testing.T) {
	content := MessageContent{
		{Type: "text", Text: "Compare these images"},
		{
			Type: "image",
			Source: &ImageSource{
				Type:      "base64",
				MediaType: "image/png",
				Data:      "image1data",
			},
		},
		{
			Type: "image",
			Source: &ImageSource{
				Type:      "base64",
				MediaType: "image/jpeg",
				Data:      "image2data",
			},
		},
	}

	data, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("failed to marshal content: %v", err)
	}

	var unmarshaled MessageContent
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal content: %v", err)
	}

	// Should have 3 blocks: text + 2 images
	if len(unmarshaled) != 3 {
		t.Fatalf("expected 3 content blocks, got %d", len(unmarshaled))
	}

	// Verify first is text
	if unmarshaled[0].Type != "text" {
		t.Errorf("expected first block to be text, got %s", unmarshaled[0].Type)
	}

	// Verify next two are images
	imageCount := 0
	for i := 1; i < 3; i++ {
		if unmarshaled[i].Type == "image" {
			imageCount++
			if unmarshaled[i].Source == nil {
				t.Errorf("image %d has nil source", i)
			}
		}
	}

	if imageCount != 2 {
		t.Errorf("expected 2 images, got %d", imageCount)
	}
}

// TestImageWithToolUse tests that images can coexist with tool_use blocks.
func TestImageWithToolUse(t *testing.T) {
	messages := []Message{
		{
			Role: RoleUser,
			Content: MessageContent{
				{Type: "text", Text: "Analyze this image"},
				{
					Type: "image",
					Source: &ImageSource{
						Type:      "base64",
						MediaType: "image/webp",
						Data:      "webpdata",
					},
				},
			},
		},
		{
			Role: RoleAssistant,
			Content: MessageContent{
				{
					Type:  "tool_use",
					ID:    "toolu_123",
					Name:  "analyze_image",
					Input: json.RawMessage(`{"image_data": "..."}`),
				},
			},
		},
	}

	req := &Request{
		Model:     "claude-3-5-sonnet-20241022",
		MaxTokens: 4096,
		Messages:  messages,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("failed to marshal request: %v", err)
	}

	var unmarshaled Request
	if err := json.Unmarshal(data, &unmarshaled); err != nil {
		t.Fatalf("failed to unmarshal request: %v", err)
	}

	// Verify first message has text and image
	if len(unmarshaled.Messages[0].Content) != 2 {
		t.Fatalf("expected 2 content blocks in first message, got %d", len(unmarshaled.Messages[0].Content))
	}

	// Verify second message has tool_use
	if len(unmarshaled.Messages[1].Content) != 1 {
		t.Fatalf("expected 1 content block in second message, got %d", len(unmarshaled.Messages[1].Content))
	}

	if unmarshaled.Messages[1].Content[0].Type != "tool_use" {
		t.Errorf("expected tool_use block, got %s", unmarshaled.Messages[1].Content[0].Type)
	}
}
