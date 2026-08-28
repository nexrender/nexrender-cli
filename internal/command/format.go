package command

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func printJSON(w io.Writer, value any) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func responseHeaders(response *http.Response) http.Header {
	if response == nil {
		return http.Header{}
	}
	return response.Header
}

func stringPointer(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func floatPointer(value *float32) string {
	if value == nil {
		return ""
	}
	return fmt.Sprintf("%.1f", *value)
}
