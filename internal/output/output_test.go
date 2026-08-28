package output

import (
	"bytes"
	"strings"
	"testing"

	"github.com/nexrender/nexrender-cli/internal/clierr"
)

func TestJSONEnvelope(t *testing.T) {
	var out bytes.Buffer
	printer := Printer{Out: &out, JSON: true}
	if err := printer.Success(map[string]int{"count": 2}, "two", nil); err != nil {
		t.Fatal(err)
	}
	text := out.String()
	for _, expected := range []string{`"ok": true`, `"count": 2`, `"summary": "two"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("output %q does not contain %q", text, expected)
		}
	}
}

func TestFailureIgnoresSuccessJQFilter(t *testing.T) {
	var out bytes.Buffer
	printer := Printer{Out: &out, JQ: ".data.id"}
	if err := printer.Failure(clierr.New("not_found", "missing", clierr.ExitNotFound)); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"code": "not_found"`) {
		t.Fatalf("unexpected error output: %s", out.String())
	}
}

func TestJQFiltering(t *testing.T) {
	var out bytes.Buffer
	printer := Printer{Out: &out, JQ: ".data.name"}
	if err := printer.Success(map[string]string{"name": "render"}, "", nil); err != nil {
		t.Fatal(err)
	}
	if out.String() != "render\n" {
		t.Fatalf("unexpected jq output: %q", out.String())
	}
}
