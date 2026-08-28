package client

import "testing"

func TestV3Root(t *testing.T) {
	tests := map[string]string{
		"https://api.nexrender.com/api/v2": "https://api.nexrender.com/api",
		"http://localhost:3000/api/v2/":    "http://localhost:3000/api",
		"http://localhost:3000/custom":     "http://localhost:3000/custom",
	}
	for input, expected := range tests {
		if actual := v3Root(input); actual != expected {
			t.Fatalf("v3Root(%q) = %q, want %q", input, actual, expected)
		}
	}
}
