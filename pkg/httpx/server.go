package httpx

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/murkio-dev/vault/internal/entities"
)

// Limites do servidor HTTP. Os timeouts protegem contra Slowloris e contra
// cliente lento, mesmo quando o servidor é alcançado sem o Caddy na frente
// (infra/Caddyfile); TLS, headers de segurança, rate limit e log de acesso são do
// Caddy. MaxBodyBytes é o teto do corpo de uma requisição e MaxHeaderBytes o dos
// cabeçalhos. ShutdownTimeout é o prazo do desligamento gracioso e cobre o
// WriteTimeout com folga: uma requisição que já começou a responder termina
// antes de o servidor ser cortado, e o que passar disso é encerrado à força para
// o deploy nunca ficar preso esperando um cliente lento.
const (
	ReadHeaderTimeout = 5 * time.Second
	ReadTimeout       = 15 * time.Second
	WriteTimeout      = 15 * time.Second
	IdleTimeout       = 60 * time.Second
	MaxHeaderBytes    = 1 << 20
	MaxBodyBytes      = 64 << 10
	ShutdownTimeout   = 20 * time.Second
)

// ServerInput reúne o que o NewServer precisa. Service é o nome que sai nos
// logs, Port é só o número da porta (sem ":"), Logs é o logger da aplicação e
// Handler é o roteador já com a autenticação.
type ServerInput struct {
	Service string
	Port    string
	Logs    *entities.Domain
	Handler http.Handler
}

// Server é o servidor HTTP pronto para subir: os limites acima, o teto de corpo
// e o recovery de panic já aplicados, e o log de boot embutido. Implementa
// graceful.Server (Serve e Shutdown).
type Server struct {
	srv      *http.Server
	listener net.Listener
	service  string
	logs     *entities.Domain
}

// NewServer abre o listener em todas as interfaces na porta pedida e monta o
// servidor em volta do Handler: primeiro o Recovery (panic vira 500 em RFC 9457
// e uma linha de log), depois o teto de corpo de MaxBodyBytes, depois o Handler.
// A porta em uso, uma porta inválida e um Logs ou Handler nil devolvem erro aqui,
// antes de o serviço dizer que subiu. A porta "0" escolhe uma livre (usada nos
// testes). Exemplo: NewServer(ServerInput{Service: "vault", Port: "8083", Logs:
// logs, Handler: mux}) escuta em :8083 e só começa a atender quando o Serve rodar.
func NewServer(in ServerInput) (*Server, error) {

	if err := AssertNotNilParams(map[string]any{"input.Logs": in.Logs, "input.Handler": in.Handler}); err != nil {
		return nil, err
	}

	listener, err := net.Listen("tcp", ":"+in.Port)
	if err != nil {
		return nil, fmt.Errorf("http: %w", err)
	}

	return &Server{
		srv: &http.Server{
			Handler:           Recovery(in.Service, in.Logs, http.MaxBytesHandler(in.Handler, MaxBodyBytes)),
			ReadHeaderTimeout: ReadHeaderTimeout,
			ReadTimeout:       ReadTimeout,
			WriteTimeout:      WriteTimeout,
			IdleTimeout:       IdleTimeout,
			MaxHeaderBytes:    MaxHeaderBytes,
		},
		listener: listener,
		service:  in.Service,
		logs:     in.Logs,
	}, nil
}

func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

// Serve registra no log que o servidor está escutando e atende até o Shutdown.
// Bloqueia enquanto o servidor roda. Devolve nil quando a parada veio do
// Shutdown (http.ErrServerClosed não é falha) e o erro de qualquer outra
// parada. Exemplo: com o servidor em :8083, grava "http listening on [::]:8083"
// e só volta depois de s.Shutdown(ctx).
func (s *Server) Serve() error {

	_ = s.logs.Boot(entities.Logs{Service: s.service, Message: "http listening on " + s.Addr()})

	if err := s.srv.Serve(s.listener); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

// Shutdown para de aceitar conexões novas e deixa as requisições em andamento
// terminarem. Se o ctx vencer antes, devolve o erro dele e as conexões que
// sobraram são cortadas por quem chamou (o graceful.Run passa um ctx com prazo
// de ShutdownTimeout).
func (s *Server) Shutdown(ctx context.Context) error {
	return s.srv.Shutdown(ctx)
}
