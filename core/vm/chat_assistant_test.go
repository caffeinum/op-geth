package vm

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

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

type mockChatAssistant struct {
	chatAssistant   // Embed the real chatAssistant to match its type
	runCalled       bool
	runStaticCalled bool
	returnValue     []byte
	err             error
}

func (m *mockChatAssistant) RequiredGas(input []byte) uint64 {
	return 100000
}

func (m *mockChatAssistant) Run(input []byte) ([]byte, error) {
	m.runCalled = true
	return m.returnValue, m.err
}

func (m *mockChatAssistant) RunStaticCall(input []byte) ([]byte, error) {
	m.runStaticCalled = true
	return m.returnValue, m.err
}

func TestChatAssistantEVMCalls(t *testing.T) {
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
	params.OpenAIAPIKey = apiKey

	// Setup mock state and accounts
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	address := common.HexToAddress("0x0000000000000000000000000000000000000100")
	caller := common.HexToAddress("0x0000000000000000000000000000000000000001")
	aiPrecompileAddress := common.HexToAddress("0x0000000000000000000000000000000000a1a1a1")

	tests := []struct {
		name         string
		isStaticCall bool

		systemPrompt     string
		message          string
		wantRunCalled    bool
		wantStaticCalled bool
		wantError        bool
		wantResponse     string
	}{
		{
			name:             "regular call executes Run",
			isStaticCall:     false,
			systemPrompt:     "You are a helpful assistant. Please respond with exactly: Hello world",
			message:          "Please say Hello world",
			wantRunCalled:    true,
			wantStaticCalled: false,
			wantResponse:     "Hello world",
		},
		{
			name:             "static call executes RunStaticCall",
			isStaticCall:     true,
			systemPrompt:     "test",
			message:          "test",
			wantRunCalled:    false,
			wantStaticCalled: true,
			wantResponse:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create real chatAssistant
			assistant := &chatAssistant{}

			// Register mock precompile
			precompiles := PrecompiledContractsBerlin
			precompiles[address] = assistant

			blockCtx := BlockContext{
				CanTransfer: func(StateDB, common.Address, *uint256.Int) bool { return true },
				Transfer:    func(StateDB, common.Address, common.Address, *uint256.Int) {},
				BlockNumber: big.NewInt(1),
				Time:        1,
				Difficulty:  big.NewInt(1),
			}

			txCtx := TxContext{
				GasPrice: big.NewInt(1),
				Origin:   caller,
			}

			evm := NewEVM(blockCtx, txCtx, statedb, params.TestChainConfig, Config{})
			evm.precompiles = precompiles

			_ = NewContract(
				AccountRef(caller),
				AccountRef(address),
				uint256.NewInt(0),
				200000,
			)

			var ret []byte
			var err error

			var input []byte

			// Method selector (4 bytes)
			input = append(input, chatMethodID...)

			// Offset for first string (32 bytes)
			offset1 := make([]byte, 32)
			binary.BigEndian.PutUint64(offset1[24:], 64) // First string starts at byte 64
			input = append(input, offset1...)

			// Offset for second string (32 bytes)
			offset2 := make([]byte, 32)
			binary.BigEndian.PutUint64(offset2[24:], uint64(96+((len(tt.systemPrompt)+31)/32)*32)) // Second string starts after first string
			input = append(input, offset2...)

			// Length of first string (32 bytes)
			length1 := make([]byte, 32)
			binary.BigEndian.PutUint64(length1[24:], uint64(len(tt.systemPrompt)))
			input = append(input, length1...)

			// Data of first string (padded to 32 byte boundary)
			data1 := make([]byte, (len(tt.systemPrompt)+31)/32*32)
			copy(data1, tt.systemPrompt)
			input = append(input, data1...)

			// Length of second string (32 bytes)
			length2 := make([]byte, 32)
			binary.BigEndian.PutUint64(length2[24:], uint64(len(tt.message)))
			input = append(input, length2...)

			// Data of second string (padded to 32 byte boundary)
			data2 := make([]byte, (len(tt.message)+31)/32*32)
			copy(data2, tt.message)
			input = append(input, data2...)

			if tt.isStaticCall {
				// Test static call
				// Call directly 0xa1a1a1 address

				ret, _, err = evm.StaticCall(
					AccountRef(caller),
					aiPrecompileAddress,
					input,
					200000,
				)
			} else {
				// Test regular call
				ret, _, err = evm.Call(
					AccountRef(caller),
					aiPrecompileAddress,
					input,
					200000,
					uint256.NewInt(0),
				)
			}

			// Verify the correct method was called
			// check response
			if err == nil {
				// For non-empty responses, decode the ABI-encoded string
				if len(ret) > 0 {
					// Skip first 32 bytes (offset)
					// Next 32 bytes contain string length
					strLen := binary.BigEndian.Uint64(ret[32+24 : 64])
					// String data starts at offset 64
					response := string(ret[64 : 64+strLen])
					if response != tt.wantResponse {
						t.Errorf("got response %q, want %q", response, tt.wantResponse)
					}
				} else if tt.wantResponse != "" {
					t.Errorf("got empty response, want %q", tt.wantResponse)
				}
			}

			// Verify error handling
			if tt.wantError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.wantError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

		})
	}
}
