package httpx

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"

	"go.yaml.in/yaml/v2"
)

type responseFormat struct {
	contentType string
	mediaTypes  []string
	render      func(output any) ([]byte, error)
}

var formats = []responseFormat{
	{"application/json", []string{"application/json"}, renderJSON},
	{"application/xml; charset=utf-8", []string{"application/xml", "text/xml"}, renderXML},
	{"application/x-yaml; charset=utf-8", []string{"application/x-yaml", "application/yaml", "text/yaml"}, renderYAML},
	{"text/plain; charset=utf-8", []string{"text/plain"}, renderTable},
}

// negotiateFormat escolhe o formato da resposta a partir do header Accept.
// Recebe o valor cru do header (lista separada por vírgula, com parâmetros
// opcionais como q=0.8) e devolve o formato registrado em formats com maior q.
// Tipos desconhecidos, q <= 0 e Accept vazio caem no padrão (formats[0], JSON).
// Em empate de q vence o que aparece primeiro; não há suporte a curingas.
// Exemplo: "application/xml;q=0.5, application/json" devolve JSON e
// "text/plain" devolve a tabela de texto.
func negotiateFormat(accept string) responseFormat {
	best := formats[0]
	bestQ := -1.0

	for _, part := range strings.Split(accept, ",") {
		fields := strings.Split(strings.TrimSpace(part), ";")
		mediaType := strings.ToLower(strings.TrimSpace(fields[0]))
		if mediaType == "" {
			continue
		}

		q := 1.0
		for _, param := range fields[1:] {
			if k, v, ok := strings.Cut(strings.TrimSpace(param), "="); ok && strings.EqualFold(k, "q") {
				if parsed, err := strconv.ParseFloat(v, 64); err == nil {
					q = parsed
				}
			}
		}
		if q <= 0 || q <= bestQ {
			continue
		}

		for _, f := range formats {
			for _, mt := range f.mediaTypes {
				if mt == mediaType {
					best, bestQ = f, q
				}
			}
		}
	}

	return best
}

func renderJSON(output any) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(output); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func normalize(output any) (any, error) {
	raw, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()

	var generic any
	if err := dec.Decode(&generic); err != nil {
		return nil, err
	}
	return convertNumbers(generic), nil
}

func convertNumbers(v any) any {
	switch t := v.(type) {
	case json.Number:
		if i, err := t.Int64(); err == nil {
			return i
		}
		f, _ := t.Float64()
		return f
	case map[string]any:
		for k, val := range t {
			t[k] = convertNumbers(val)
		}
	case []any:
		for i, val := range t {
			t[i] = convertNumbers(val)
		}
	}
	return v
}

func renderYAML(output any) ([]byte, error) {
	generic, err := normalize(output)
	if err != nil {
		return nil, err
	}
	return yaml.Marshal(generic)
}

func renderXML(output any) ([]byte, error) {
	generic, err := normalize(output)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	buf.WriteString(xml.Header)

	enc := xml.NewEncoder(&buf)
	enc.Indent("", "  ")
	if err := writeXML(enc, "response", generic); err != nil {
		return nil, err
	}
	if err := enc.Flush(); err != nil {
		return nil, err
	}
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func writeXML(enc *xml.Encoder, name string, v any) error {
	start := xml.StartElement{Name: xml.Name{Local: name}}

	switch t := v.(type) {
	case map[string]any:
		if err := enc.EncodeToken(start); err != nil {
			return err
		}
		for _, key := range sortedKeys(t) {
			if list, ok := t[key].([]any); ok {
				for _, item := range list {
					if err := writeXML(enc, key, item); err != nil {
						return err
					}
				}
				continue
			}
			if err := writeXML(enc, key, t[key]); err != nil {
				return err
			}
		}
		return enc.EncodeToken(start.End())
	case []any:
		if err := enc.EncodeToken(start); err != nil {
			return err
		}
		for _, item := range t {
			if err := writeXML(enc, "item", item); err != nil {
				return err
			}
		}
		return enc.EncodeToken(start.End())
	case nil:
		if err := enc.EncodeToken(start); err != nil {
			return err
		}
		return enc.EncodeToken(start.End())
	default:
		return enc.EncodeElement(cell(t), start)
	}
}

type section struct {
	name string
	rows [][]string
}

func sections(output any) ([]section, error) {
	generic, err := normalize(output)
	if err != nil {
		return nil, err
	}

	root, ok := generic.(map[string]any)
	if !ok {
		return []section{{name: "data", rows: toRows(generic)}}, nil
	}

	var result []section
	for _, key := range []string{"data", "error", "errors", "page"} {
		if val, found := root[key]; found && val != nil {
			result = append(result, section{name: key, rows: toRows(val)})
		}
	}
	return result, nil
}

func toRows(v any) [][]string {
	switch t := v.(type) {
	case []any:
		maps := make([]map[string]any, 0, len(t))
		for _, item := range t {
			m, ok := item.(map[string]any)
			if !ok {
				rows := [][]string{{"value"}}
				for _, s := range t {
					rows = append(rows, []string{cell(s)})
				}
				return rows
			}
			maps = append(maps, m)
		}

		columns := map[string]struct{}{}
		for _, m := range maps {
			for k := range m {
				columns[k] = struct{}{}
			}
		}
		header := sortedKeys(columns)

		rows := [][]string{header}
		for _, m := range maps {
			row := make([]string, len(header))
			for i, col := range header {
				row[i] = cell(m[col])
			}
			rows = append(rows, row)
		}
		return rows
	case map[string]any:
		rows := [][]string{{"field", "value"}}
		for _, k := range sortedKeys(t) {
			rows = append(rows, []string{k, cell(t[k])})
		}
		return rows
	default:
		return [][]string{{"value"}, {cell(t)}}
	}
}

func renderTable(output any) ([]byte, error) {
	secs, err := sections(output)
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	for i, s := range secs {
		if i > 0 {
			buf.WriteString("\n")
		}
		if len(secs) > 1 {
			buf.WriteString(strings.ToUpper(s.name) + "\n")
		}

		tw := tabwriter.NewWriter(&buf, 0, 0, 2, ' ', 0)
		for _, row := range s.rows {
			fmt.Fprintln(tw, strings.Join(row, "\t"))
		}
		if err := tw.Flush(); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func cell(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case map[string]any, []any:
		b, _ := json.Marshal(t)
		return string(b)
	default:
		return fmt.Sprint(t)
	}
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
