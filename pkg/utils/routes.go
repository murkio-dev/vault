package utils

import "net/http"

// Handle registra pattern ("GET /api/v1/x") e a variante com barra final
// ("GET /api/v1/x/{$}"), como o api.md manda para toda rota: o cliente que
// termina o caminho com "/" cai no mesmo handler em vez de num 404.
//
// Entrada: o padrão do http.ServeMux com método, sem barra final. Saída: nada;
// registra duas rotas no mux. Um padrão que já termina em "/" ou em "{$}" é
// registrado uma vez só. Exemplo: Handle(mux, "GET /api/v1/applications/{app}/members", h)
// atende "/api/v1/applications/<id>/members" e "/api/v1/applications/<id>/members/".
func Handle(mux *http.ServeMux, pattern string, handler http.HandlerFunc) {
	mux.HandleFunc(pattern, handler)

	if last := pattern[len(pattern)-1]; last == '/' || last == '}' && len(pattern) >= 3 && pattern[len(pattern)-3:] == "{$}" {
		return
	}
	mux.HandleFunc(pattern+"/{$}", handler)
}
