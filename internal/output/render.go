package output

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

type Format string

const (
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatTable Format = "table"
)

func ValidateFormat(format Format) error {
	switch format {
	case "", FormatJSON, FormatYAML, FormatTable:
		return nil
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

type Renderer struct {
	stdout io.Writer
	notice map[string]any
}

func NewRenderer(stdout io.Writer) *Renderer {
	return &Renderer{stdout: stdout}
}

func (r *Renderer) WithNotice(notice map[string]any) *Renderer {
	r.notice = notice
	return r
}

func (r *Renderer) Render(format Format, value any) error {
	if err := ValidateFormat(format); err != nil {
		return err
	}
	normalized, err := normalizeValue(value)
	if err != nil {
		return err
	}

	switch format {
	case "", FormatJSON:
		return r.renderJSON(normalized)
	case FormatYAML:
		return r.renderYAML(normalized, 0)
	case FormatTable:
		return r.renderTable(normalized)
	default:
		return fmt.Errorf("unsupported output format %q", format)
	}
}

func (r *Renderer) RenderRaw(raw []byte) error {
	_, err := r.stdout.Write(raw)
	return err
}

func (r *Renderer) renderJSON(value any) error {
	injectNotice(value, r.notice)
	var data bytes.Buffer
	encoder := json.NewEncoder(&data)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return fmt.Errorf("marshal json output: %w", err)
	}
	_, err := r.stdout.Write(data.Bytes())
	return err
}

func injectNotice(value any, notice map[string]any) {
	if len(notice) == 0 {
		return
	}
	object, ok := value.(map[string]any)
	if !ok {
		return
	}
	existing, ok := object["_notice"].(map[string]any)
	if !ok {
		existing = map[string]any{}
		object["_notice"] = existing
	}
	for key, val := range notice {
		existing[key] = val
	}
}

func (r *Renderer) renderTable(value any) error {
	writer := tabwriter.NewWriter(r.stdout, 0, 0, 2, ' ', 0)

	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		_, _ = fmt.Fprintln(writer, "KEY\tVALUE")
		for _, key := range keys {
			_, _ = fmt.Fprintf(writer, "%s\t%s\n", key, tableValueString(typed[key]))
		}
		return writer.Flush()
	case []map[string]any:
		return renderTableRows(writer, typed)
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for _, row := range typed {
			asMap, ok := row.(map[string]any)
			if !ok {
				return r.renderJSON(value)
			}
			rows = append(rows, asMap)
		}
		return renderTableRows(writer, rows)
	default:
		return r.renderJSON(value)
	}
}

func renderTableRows(writer *tabwriter.Writer, rows []map[string]any) error {
	columnSet := map[string]struct{}{}
	for _, row := range rows {
		for key := range row {
			columnSet[key] = struct{}{}
		}
	}

	columns := make([]string, 0, len(columnSet))
	for key := range columnSet {
		columns = append(columns, key)
	}
	sort.Strings(columns)

	_, _ = fmt.Fprintln(writer, strings.Join(columns, "\t"))
	for _, row := range rows {
		values := make([]string, 0, len(columns))
		for _, column := range columns {
			values = append(values, tableValueString(row[column]))
		}
		_, _ = fmt.Fprintln(writer, strings.Join(values, "\t"))
	}

	return writer.Flush()
}

func (r *Renderer) renderYAML(value any, indent int) error {
	switch typed := value.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for key := range typed {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if isScalar(typed[key]) {
				if _, err := fmt.Fprintf(r.stdout, "%s%s: %s\n", strings.Repeat(" ", indent), key, yamlScalarString(typed[key])); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(r.stdout, "%s%s:\n", strings.Repeat(" ", indent), key); err != nil {
				return err
			}
			if err := r.renderYAML(typed[key], indent+2); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range typed {
			if isScalar(item) {
				if _, err := fmt.Fprintf(r.stdout, "%s- %s\n", strings.Repeat(" ", indent), yamlScalarString(item)); err != nil {
					return err
				}
				continue
			}
			if _, err := fmt.Fprintf(r.stdout, "%s-\n", strings.Repeat(" ", indent)); err != nil {
				return err
			}
			if err := r.renderYAML(item, indent+2); err != nil {
				return err
			}
		}
	case []map[string]any:
		items := make([]any, 0, len(typed))
		for _, item := range typed {
			items = append(items, item)
		}
		return r.renderYAML(items, indent)
	default:
		if _, err := fmt.Fprintf(r.stdout, "%s%s\n", strings.Repeat(" ", indent), yamlScalarString(typed)); err != nil {
			return err
		}
	}

	return nil
}

func normalizeValue(value any) (any, error) {
	switch typed := value.(type) {
	case json.RawMessage:
		return decodeJSONBytes([]byte(typed))
	case []byte:
		return decodeJSONBytes(typed)
	default:
		return value, nil
	}
}

func decodeJSONBytes(data []byte) (any, error) {
	if len(data) == 0 {
		return map[string]any{}, nil
	}
	var value any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return string(data), nil
	}
	return value, nil
}

func tableValueString(value any) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	default:
		data, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprintf("%v", typed)
		}
		return string(data)
	}
}

func isScalar(value any) bool {
	switch value.(type) {
	case nil, string, bool,
		int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64, json.Number:
		return true
	default:
		return false
	}
}

func yamlScalarString(value any) string {
	switch typed := value.(type) {
	case nil:
		return "null"
	case string:
		if typed == "" {
			return `""`
		}
		return strconv.Quote(typed)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case json.Number:
		return typed.String()
	default:
		return fmt.Sprintf("%v", typed)
	}
}
