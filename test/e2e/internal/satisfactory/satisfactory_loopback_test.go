package main

import (
	"testing"
)

func TestIsLoopbackHost(t *testing.T) {
	tests := []struct {
		name     string
		host     string
		expected bool
	}{
		// Loopback cases: should return true
		{
			name:     "localhost literal",
			host:     "localhost",
			expected: true,
		},
		{
			name:     "localhost uppercase",
			host:     "LOCALHOST",
			expected: true,
		},
		{
			name:     "localhost mixed case",
			host:     "LocaLHost",
			expected: true,
		},
		{
			name:     "IPv4 loopback",
			host:     "127.0.0.1",
			expected: true,
		},
		{
			name:     "IPv4 loopback other octet",
			host:     "127.0.0.2",
			expected: true,
		},
		{
			name:     "IPv6 loopback short",
			host:     "::1",
			expected: true,
		},
		{
			name:     "IPv6 loopback expanded",
			host:     "0:0:0:0:0:0:0:1",
			expected: true,
		},
		{
			name:     "IPv6 loopback compressed",
			host:     "::1",
			expected: true,
		},

		// Non-loopback cases: should return false
		{
			name:     "public IPv4 8.8.8.8",
			host:     "8.8.8.8",
			expected: false,
		},
		{
			name:     "public IPv4 1.1.1.1",
			host:     "1.1.1.1",
			expected: false,
		},
		{
			name:     "IPv4 all zeros",
			host:     "0.0.0.0",
			expected: false,
		},
		{
			name:     "public hostname",
			host:     "example.com",
			expected: false,
		},
		{
			name:     "private IPv4 10.0.0.1",
			host:     "10.0.0.1",
			expected: false,
		},
		{
			name:     "private IPv4 172.16.0.1",
			host:     "172.16.0.1",
			expected: false,
		},
		{
			name:     "private IPv4 192.168.1.1",
			host:     "192.168.1.1",
			expected: false,
		},
		{
			name:     "IPv6 all zeros",
			host:     "::",
			expected: false,
		},
		{
			name:     "IPv6 public address",
			host:     "2001:db8::1",
			expected: false,
		},
		{
			name:     "invalid IP",
			host:     "invalid.host.invalid",
			expected: false,
		},
		{
			name:     "empty string",
			host:     "",
			expected: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := isLoopbackHost(tc.host)
			if result != tc.expected {
				t.Errorf("isLoopbackHost(%q) = %v, expected %v", tc.host, result, tc.expected)
			}
		})
	}
}

func TestQueryServerStateRejectsNonLoopback(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{
			name: "public IP 8.8.8.8 with port",
			addr: "8.8.8.8:7777",
		},
		{
			name: "public IP 0.0.0.0 with port",
			addr: "0.0.0.0:7777",
		},
		{
			name: "public hostname with port",
			addr: "example.com:7777",
		},
		{
			name: "private IP 192.168.1.1 with port",
			addr: "192.168.1.1:7777",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := queryServerState(nil, tc.addr)
			if err == nil {
				t.Errorf("queryServerState(%q) should reject non-loopback address, but got nil error", tc.addr)
			}
		})
	}
}

func TestQueryServerStateAcceptsLoopback(t *testing.T) {
	tests := []struct {
		name string
		addr string
	}{
		{
			name: "localhost with port",
			addr: "localhost:7777",
		},
		{
			name: "localhost uppercase with port",
			addr: "LOCALHOST:7777",
		},
		{
			name: "IPv4 loopback with port",
			addr: "127.0.0.1:7777",
		},
		{
			name: "IPv4 loopback 127.0.0.2 with port",
			addr: "127.0.0.2:7777",
		},
		{
			name: "IPv6 loopback with port",
			addr: "[::1]:7777",
		},
		{
			name: "bare localhost no port",
			addr: "localhost",
		},
		{
			name: "bare IPv4 loopback no port",
			addr: "127.0.0.1",
		},
		{
			name: "bare IPv6 loopback no port",
			addr: "::1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := queryServerState(nil, tc.addr)
			// We expect an error (context nil), but NOT a loopback rejection error.
			// The loopback guard should pass, and only the nil context or missing server should cause the error.
			if err != nil {
				// Check that the error is NOT about refusing non-loopback connection.
				errMsg := err.Error()
				if errMsg == "refusing non-loopback connection" || len(errMsg) > 0 && errMsg[:len("refusing non-loopback")] == "refusing non-loopback" {
					t.Errorf("queryServerState(%q) rejected loopback address: %v", tc.addr, err)
				}
				// Other errors (context, network, etc.) are expected and acceptable.
			}
		})
	}
}
