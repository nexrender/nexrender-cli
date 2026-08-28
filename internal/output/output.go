package output

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/itchyny/gojq"
	"github.com/nexrender/nexrender-cli/internal/clierr"
)

type Envelope struct {
	OK          bool     `json:"ok"`
	Data        any      `json:"data,omitempty"`
	Summary     string   `json:"summary,omitempty"`
	Breadcrumbs []string `json:"breadcrumbs"`
}

type ErrorEnvelope struct {
	OK    bool          `json:"ok"`
	Error *clierr.Error `json:"error"`
}

type Printer struct {
	Out   io.Writer
	JSON  bool
	Quiet bool
	JQ    string
}

type HumanFunc func(io.Writer) error

func (p Printer) Success(data any, summary string, human HumanFunc) error {
	envelope := Envelope{OK: true, Data: data, Summary: summary, Breadcrumbs: []string{}}
	if p.JSON || p.JQ != "" {
		var value any = envelope
		if p.Quiet {
			value = data
		}
		return p.printStructured(value)
	}
	if human != nil {
		return human(p.Out)
	}
	if summary != "" {
		_, err := fmt.Fprintln(p.Out, summary)
		return err
	}
	return p.printStructured(data)
}

func (p Printer) Failure(err error) error {
	structured := clierr.As(err)
	if p.JSON || p.JQ != "" {
		p.JQ = ""
		p.Quiet = false
		return p.printStructured(ErrorEnvelope{OK: false, Error: structured})
	}
	if _, writeErr := fmt.Fprintf(p.Out, "Error: %s\n", structured.Message); writeErr != nil {
		return writeErr
	}
	if structured.Hint != "" {
		_, writeErr := fmt.Fprintf(p.Out, "Hint: %s\n", structured.Hint)
		return writeErr
	}
	return nil
}

func (p Printer) printStructured(value any) error {
	if p.JQ != "" {
		query, err := gojq.Parse(p.JQ)
		if err != nil {
			return clierr.Usage("invalid --jq expression: " + err.Error())
		}
		normalized, err := normalize(value)
		if err != nil {
			return err
		}
		iter := query.Run(normalized)
		encoder := json.NewEncoder(p.Out)
		encoder.SetIndent("", "  ")
		for {
			result, ok := iter.Next()
			if !ok {
				return nil
			}
			if err, ok := result.(error); ok {
				return clierr.Usage("--jq evaluation failed: " + err.Error())
			}
			if text, ok := result.(string); ok {
				if _, err := fmt.Fprintln(p.Out, text); err != nil {
					return err
				}
				continue
			}
			if err := encoder.Encode(result); err != nil {
				return err
			}
		}
	}
	encoder := json.NewEncoder(p.Out)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}

func normalize(value any) (any, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	var result any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}
	return result, nil
}
