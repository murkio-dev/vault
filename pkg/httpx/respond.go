package httpx

import (
	"net/http"
	"reflect"

	"github.com/murkio-dev/vault/internal/entities"
)

func Respond(w http.ResponseWriter, r *http.Request, output any, successStatus int) {
	respond(w, r, output, successStatus, true)
}

// RespondList responde como Respond, mas nunca desembrulha item único:
// listagens sempre voltam como array, mesmo com um só resultado.
// Recebe o output do use case (com Data, Error e Page), o request (usado para
// negociar o formato pelo header Accept) e o status de sucesso.
// Exemplo: uma página com 1 aplicação responde {"data":[{...}],"page":{...}},
// enquanto Respond responderia {"data":{...}}.
// Em caso de erro, o status vem do primeiro erro do output, como em Respond.
func RespondList(w http.ResponseWriter, r *http.Request, output any, successStatus int) {
	respond(w, r, output, successStatus, false)
}

func respond(w http.ResponseWriter, r *http.Request, output any, successStatus int, unwrap bool) {
	errs, hasData := inspectOutput(output)

	status := successStatus
	switch {
	case len(errs) > 0 && hasData:
		status = http.StatusMultiStatus
	case len(errs) > 0:
		status = errs[0].HTTPStatus()
	case status == http.StatusNoContent:
		w.WriteHeader(status)
		return
	}

	if unwrap {
		output = unwrapSingle(output)
	}
	write(w, r, status, output)
}

// unwrapSingle remove o wrapper de lista quando a resposta tem uma única
// entidade, para o cliente receber um objeto em vez de um array de um item.
// Recebe o output do use case (qualquer struct serializável) e o normaliza
// para um map genérico; se isso falhar ou o formato não for um objeto, devolve
// o output original sem alterações.
// Exemplo: {"data":[{"id":1}]} vira {"data":{"id":1}} e {"errors":[{...}]}
// vira {"error":{...}}.
// Respostas mistas (data e errors juntos) e lotes reais (2 ou mais itens)
// mantêm as listas. Listagens não passam por aqui (veja RespondList).
func unwrapSingle(output any) any {
	generic, err := normalize(output)
	if err != nil {
		return output
	}
	root, ok := generic.(map[string]any)
	if !ok {
		return output
	}

	data, _ := root["data"].([]any)
	errs, _ := root["errors"].([]any)
	changed := false

	if len(data) == 1 && len(errs) == 0 {
		root["data"] = data[0]
		changed = true
	}
	if len(errs) == 1 && len(data) == 0 {
		root["error"] = errs[0]
		delete(root, "errors")
		changed = true
	}

	if !changed {
		return output
	}
	return root
}

func RespondError(w http.ResponseWriter, r *http.Request, err entities.Error) {
	if err.Instance == "" {
		err.Instance = r.URL.Path
	}
	write(w, r, err.HTTPStatus(), struct {
		Error entities.Error `json:"error"`
	}{Error: err})
}

func write(w http.ResponseWriter, r *http.Request, status int, output any) {
	format := negotiateFormat(r.Header.Get("Accept"))

	body, err := format.render(output)
	if err != nil {
		format = formats[0]
		status = http.StatusInternalServerError
		body, _ = format.render(struct {
			Error entities.Error `json:"error"`
		}{Error: entities.InternalServerError(err, nil)})
	}

	w.Header().Set("Content-Type", format.contentType)
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func inspectOutput(output any) (errs []entities.Error, hasData bool) {
	v := reflect.ValueOf(output)
	for v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface {
		if v.IsNil() {
			return nil, false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil, false
	}

	if f := v.FieldByName("Error"); f.IsValid() {
		switch e := f.Interface().(type) {
		case *entities.Error:
			if e != nil {
				errs = append(errs, *e)
			}
		case entities.Error:
			errs = append(errs, e)
		}
	}
	if f := v.FieldByName("Errors"); f.IsValid() {
		if list, ok := f.Interface().([]entities.Error); ok {
			errs = append(errs, list...)
		}
	}

	if f := v.FieldByName("Data"); f.IsValid() {
		if f.Kind() == reflect.Slice || f.Kind() == reflect.Map {
			hasData = f.Len() > 0
		} else {
			hasData = !f.IsZero()
		}
	}

	return errs, hasData
}

// RespondJSON responde como Respond, mas SEMPRE em JSON, ignorando o Accept.
// Existe para as rotas que devolvem valor de segredo, token ou convite: YAML,
// XML e a tabela de texto são outros renderizadores sobre o mesmo dado, e cada
// um seria mais uma saída para um valor sensível que ninguém revisou.
// Entrada: o output do use case e o status de sucesso. Saída: a resposta HTTP;
// com erro, o status é o do primeiro erro, como em Respond.
// Exemplo: Accept "text/plain" recebe {"data":{"value":"..."}} em JSON.
func RespondJSON(w http.ResponseWriter, r *http.Request, output any, successStatus int) {
	respond(w, forceJSON(r), output, successStatus, true)
}

func forceJSON(r *http.Request) *http.Request {
	forced := r.Clone(r.Context())
	forced.Header.Set("Accept", "application/json")
	return forced
}
