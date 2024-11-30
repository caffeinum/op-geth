package vm

import (
	"bytes"
	"errors"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

func TestChatAssistant(t *testing.T) {
	// Skip if no API key is set
	apiKey := os.Getenv("GETH_OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping test: OPENAI_API_KEY not set")
	}

	// Save original API key and restore after tests
	originalAPIKey := params.OpenAIAPIKey
	defer func() {
		params.OpenAIAPIKey = originalAPIKey
	}()

	// Set API key for testing
	params.OpenAIAPIKey = apiKey

	tests := []struct {
		name         string
		systemPrompt string
		message      string
		wantErr      error
		isStaticCall bool
	}{
		{
			name:         "basic chat success",
			systemPrompt: "You are a helpful assistant. Please respond with exactly: Hello world",
			message:      "Please say Hello world",
			wantErr:      nil,
		},
		{
			name:         "static call returns empty",
			systemPrompt: "test",
			message:      "test",
			isStaticCall: true,
		},
		{
			name:         "static call bool returns false",
			systemPrompt: "test",
			message:      "test",
			isStaticCall: true,
		},
		{
			name:         "static call uint returns zero",
			systemPrompt: "test",
			message:      "test",
			isStaticCall: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Calculate total length needed for the input
			systemPromptLen := len(tt.systemPrompt)
			messageLen := len(tt.message)

			// Calculate padding needed
			systemPromptPadding := (32 - (systemPromptLen % 32)) % 32
			messagePadding := (32 - (messageLen % 32)) % 32

			// Total length = 4 (method ID) + 32 (offset1) + 32 (offset2) +
			//               32 (systemPrompt length) + systemPromptLen + padding +
			//               32 (message length) + messageLen + padding
			totalLength := 4 + 32 + 32 + 32 + systemPromptLen + systemPromptPadding + 32 + messageLen + messagePadding

			// Prepare the input buffer
			input := make([]byte, totalLength)

			// Copy method ID
			copy(input[0:4], chatMethodID)

			// Write offset to first string (32)
			offset1 := big.NewInt(64) // 4 + 32 + 32 (methodID + two offsets)
			copy(input[4:36], common.LeftPadBytes(offset1.Bytes(), 32))

			// Write offset to second string
			offset2 := big.NewInt(int64(64 + 32 + systemPromptLen + systemPromptPadding)) // offset1 + len + data + padding
			copy(input[36:68], common.LeftPadBytes(offset2.Bytes(), 32))

			// Write system prompt length and data
			copy(input[68:100], common.LeftPadBytes(big.NewInt(int64(systemPromptLen)).Bytes(), 32))
			copy(input[100:100+systemPromptLen], []byte(tt.systemPrompt))

			// Write message length and data
			messageOffset := 100 + systemPromptLen + systemPromptPadding
			copy(input[messageOffset:messageOffset+32], common.LeftPadBytes(big.NewInt(int64(messageLen)).Bytes(), 32))
			copy(input[messageOffset+32:messageOffset+32+messageLen], []byte(tt.message))

			c := &chatAssistant{}
			var got []byte
			var err error

			if tt.isStaticCall {
				switch {
				case bytes.Equal(input[0:4], chatMethodID):
					got, err = c.RunStaticCall(input)
					if err != nil {
						t.Errorf("chatAssistant.RunStaticCall() error = %v", err)
					}
					if len(got) != 0 {
						t.Errorf("chatAssistant.RunStaticCall() = %v, want empty", got)
					}
				case bytes.Equal(input[0:4], chatBoolMethodID):
					got, err = c.RunStaticCall(append(chatBoolMethodID, input[4:]...))
					if err != nil {
						t.Errorf("chatAssistant.RunStaticCall() error = %v", err)
					}
					if !bytes.Equal(got, []byte{0}) {
						t.Errorf("chatAssistant.RunStaticCall() = %v, want false", got)
					}
				case bytes.Equal(input[0:4], chatUintMethodID):
					got, err = c.RunStaticCall(append(chatUintMethodID, input[4:]...))
					if err != nil {
						t.Errorf("chatAssistant.RunStaticCall() error = %v", err)
					}
					if !bytes.Equal(got, make([]byte, 32)) {
						t.Errorf("chatAssistant.RunStaticCall() = %v, want zero", got)
					}
				}
				return
			}

			got, err = c.Run(input)
			if tt.wantErr != nil {
				if err == nil || !errors.Is(err, tt.wantErr) {
					t.Errorf("chatAssistant.Run() error = %v, wantErr %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Errorf("chatAssistant.Run() unexpected error = %v", err)
				return
			}

			// For the basic chat success test, verify the response contains "Hello world"
			if tt.name == "basic chat success" {
				// Skip first 64 bytes (offsets and length)
				response := string(got[64:75])
				if response != "Hello world" {
					t.Errorf("chatAssistant.Run() = %v, want 'Hello world'", response)
				}
			}
		})
	}
}
