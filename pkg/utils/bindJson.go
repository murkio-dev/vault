package utils

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"

	"io"
	"net/http"
	"strings"
	"uuid"

	"go.yaml.in/yaml/v2"
)

func BindBody[T any](r *http.Request) ([]T, error) {
	if r.Body == nil {
		return nil, errors.New("empty request body")
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, errors.New("failed to read request body")
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, errors.New("empty payload")
	}

	contentType := r.Header.Get("Content-Type")

	switch {
	case strings.Contains(contentType, "application/json"):
		return decodeJSON[T](trimmed)
	case strings.Contains(contentType, "text/xml") || strings.Contains(contentType, "application/xml"):
		return decodeXML[T](trimmed)
	case strings.Contains(contentType, "application/x-yaml") || strings.Contains(contentType, "text/yaml"):
		return decodeYAML[T](trimmed)
	}

	switch trimmed[0] {
	case '[', '{':
		return decodeJSON[T](trimmed)
	case '<':
		return decodeXML[T](trimmed)
	default:
		return decodeYAML[T](trimmed)
	}
}

func decodeJSON[T any](data []byte) ([]T, error) {
	switch data[0] {
	case '[':
		var result []T
		if err := json.Unmarshal(data, &result); err != nil {
			return nil, fmt.Errorf("invalid JSON array payload: %w", err)
		}
		return result, nil
	case '{':
		var single T
		if err := json.Unmarshal(data, &single); err != nil {
			return nil, fmt.Errorf("invalid JSON object payload: %w", err)
		}
		return []T{single}, nil
	default:
		return nil, errors.New("invalid JSON format")
	}
}

func decodeXML[T any](data []byte) ([]T, error) {
	var listWrapper struct {
		Items []T `xml:",any"`
	}
	if err := xml.Unmarshal(data, &listWrapper); err == nil && len(listWrapper.Items) > 0 {
		return listWrapper.Items, nil
	}

	var single T
	if err := xml.Unmarshal(data, &single); err != nil {
		return nil, fmt.Errorf("invalid XML payload: %w", err)
	}

	return []T{single}, nil
}

func decodeYAML[T any](data []byte) ([]T, error) {
	var list []T
	if err := yaml.Unmarshal(data, &list); err == nil && len(list) > 0 {
		return list, nil
	}

	var single T
	if err := yaml.Unmarshal(data, &single); err != nil {
		return nil, fmt.Errorf("invalid YAML payload: %w", err)
	}

	return []T{single}, nil
}

type idItem struct {
	ID uuid.UUID `json:"id" xml:"id" yaml:"id"`
}

// BindIDs extrai uma lista de ids do body, em JSON, XML ou YAML, para as
// rotas que recebem só ids (como o DELETE em lote).
// Aceita uma lista de ids ou objetos com o campo id, e também um único objeto.
// O formato é detectado pelo primeiro caractere: [ ou { (JSON), < (XML) e o
// resto como YAML. Devolve erro para body vazio, formato não suportado, payload
// malformado ou sem nenhum id.
// Exemplos válidos: ["<uuid>","<uuid>"], {"id":"<uuid>"}, [{"id":"<uuid>"}]
// e, em YAML, "- <uuid>". Não valida duplicados (isso é do controller).
func BindIDs(r *http.Request) ([]uuid.UUID, error) {
	if r.Body == nil {
		return nil, errors.New("empty request body")
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, errors.New("failed to read request body")
	}

	data := bytes.TrimSpace(body)
	if len(data) == 0 {
		return nil, errors.New("empty payload")
	}

	var ids []uuid.UUID

	switch {
	case data[0] == '[' || data[0] == '{':
		var list []uuid.UUID
		if data[0] == '[' && json.Unmarshal(data, &list) == nil {
			ids = list
			break
		}
		items, err := decodeJSON[idItem](data)
		if err != nil {
			return nil, err
		}
		ids = idsFromItems(items)
	case data[0] == '<':
		var wrapper struct {
			Items []uuid.UUID `xml:",any"`
		}
		if xml.Unmarshal(data, &wrapper) == nil && len(wrapper.Items) > 0 {
			ids = wrapper.Items
			break
		}
		items, err := decodeXML[idItem](data)
		if err != nil {
			return nil, err
		}
		ids = idsFromItems(items)
	case bytes.Contains(data, []byte(": ")) || bytes.HasPrefix(data, []byte("- ")):
		var list []uuid.UUID
		if yaml.Unmarshal(data, &list) == nil && len(list) > 0 {
			ids = list
			break
		}
		items, err := decodeYAML[idItem](data)
		if err != nil {
			return nil, err
		}
		ids = idsFromItems(items)
	default:
		return nil, errors.New("unsupported payload format, use JSON, XML or YAML")
	}

	if len(ids) == 0 {
		return nil, errors.New("no ids found in payload")
	}
	return ids, nil
}

func idsFromItems(items []idItem) []uuid.UUID {
	ids := make([]uuid.UUID, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

// BindJSON lê o corpo como JSON estrito, para as rotas com segredo ou papel.
// Ao contrário do BindBody, só aceita JSON (o Content-Type não decide, então
// YAML e XML nunca são interpretados) e recusa campo desconhecido: o cliente
// que manda created_by, member_id ou role onde não cabe recebe erro em vez de
// ver o campo ignorado em silêncio. Aceita um objeto ou uma lista e devolve
// sempre []T. Exemplo: {"name":"A","x":1} falha; {"name":"A"} vira [{A}].
func BindJSON[T any](r *http.Request) ([]T, error) {
	if r.Body == nil {
		return nil, errors.New("empty request body")
	}
	defer r.Body.Close()

	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, errors.New("failed to read request body")
	}

	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return nil, errors.New("empty payload")
	}

	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()

	switch trimmed[0] {
	case '[':
		var list []T
		if err := decoder.Decode(&list); err != nil {
			return nil, errors.New("invalid JSON array payload")
		}
		return list, nil
	case '{':
		var single T
		if err := decoder.Decode(&single); err != nil {
			return nil, errors.New("invalid JSON object payload")
		}
		return []T{single}, nil
	}

	return nil, errors.New("invalid JSON format")
}
