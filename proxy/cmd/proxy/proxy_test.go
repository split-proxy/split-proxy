package main

import (
	"testing"
	"net/http"
)

func TestParseConnectTarget(t *testing.T) {
	tests := []struct {
		name        string
		requestURI  string
		host        string
		wantHost    string
		wantPort    string
		wantErr     bool
	}{
		{
			name:       "hostname with port",
			requestURI: "example.com:443",
			wantHost:   "example.com",
			wantPort:   "443",
		},
		{
			name:       "hostname from Host header",
			requestURI: "",
			host:       "example.com:443",
			wantHost:   "example.com",
			wantPort:   "443",
		},
		{
			name:       "ipv4 with port",
			requestURI: "127.0.0.1:8080",
			wantHost:   "127.0.0.1",
			wantPort:   "8080",
		},
		{
			name:       "ipv6 with port",
			requestURI: "[::1]:8080",
			wantHost:   "::1",
			wantPort:   "8080",
		},
		{
			name:       "minimum valid port",
			requestURI: "example.com:1",
			wantHost:   "example.com",
			wantPort:   "1",
		},
		{
			name:       "maximum valid port",
			requestURI: "example.com:65535",
			wantHost:   "example.com",
			wantPort:   "65535",
		},
		{
			name:       "empty target",
			requestURI: "",
			host:       "",
			wantErr:    true,
		},
		{
			name:       "missing port",
			requestURI: "example.com",
			wantErr:    true,
		},
		{
			name:       "invalid port",
			requestURI: "example.com:abc",
			wantErr:    true,
		},
		{
			name:       "port zero",
			requestURI: "example.com:0",
			wantErr:    true,
		},
		{
			name:       "port greater than 65535",
			requestURI: "example.com:65536",
			wantErr:    true,
		},
		{
			name:       "empty host",
			requestURI: ":443",
			wantErr:    true,
		},
		{
			name:       "negative port",
			requestURI: "example.com:-1",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				RequestURI: tt.requestURI,
				Host:       tt.host,
			}

			gotHost, gotPort, err := parseConnectTarget(req)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}

				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if gotHost != tt.wantHost {
				t.Errorf(
					"host = %q, want %q",
					gotHost,
					tt.wantHost,
				)
			}

			if gotPort != tt.wantPort {
				t.Errorf(
					"port = %q, want %q",
					gotPort,
					tt.wantPort,
				)
			}
		})
	}
}
