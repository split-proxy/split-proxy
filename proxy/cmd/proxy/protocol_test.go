package main

import (
	"bufio"
	"strings"
	"testing"
)

func TestDetectProtocol(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected Protocol
		wantErr  bool
	}{
		{
			name:     "SOCKS5",
			input:    "\x05\x01\x00",
			expected: ProtocolSOCKS5,
		},
		{
			name:     "TLS",
			input:    "\x16\x03\x01",
			expected: ProtocolTLS,
		},
		{
			name:     "HTTP CONNECT",
			input:    "CONNECT example.com:443 HTTP/1.1\r\n",
			expected: ProtocolHTTP,
		},
		{
			name:     "unknown protocol",
			input:    "GET / HTTP/1.1\r\n",
			expected: ProtocolUnknown,
			wantErr:  true,
		},
		{
			name:     "empty input",
			input:    "",
			expected: ProtocolUnknown,
			wantErr:  true,
		},
		{
			name:     "incomplete TLS header",
			input:    "\x16\x03",
			expected: ProtocolUnknown,
			wantErr:  true,
		},
		{
			name:     "incomplete HTTP CONNECT",
			input:    "CON",
			expected: ProtocolUnknown,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reader := bufio.NewReader(strings.NewReader(tt.input))

			got, err := detectProtocol(reader)

			if got != tt.expected {
				t.Errorf(
					"detectProtocol() = %v, want %v",
					got,
					tt.expected,
				)
			}

			if (err != nil) != tt.wantErr {
				t.Errorf(
					"detectProtocol() error = %v, wantErr %v",
					err,
					tt.wantErr,
				)
			}
		})
	}
}
