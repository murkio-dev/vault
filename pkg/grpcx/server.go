package grpcx

import (
	"context"
	"fmt"
	"net"

	"github.com/murkio-dev/vault/pkg/authn"
	"google.golang.org/grpc"
)

const (
	loopbackHost = "127.0.0.1"

	// maxMessageBytes é o teto de uma mensagem recebida, o mesmo do corpo HTTP
	// (httpx.MaxBodyBytes). O padrão do gRPC é 4 MB, muito acima do que um lote de
	// aplicações ou um pedido de ambiente precisa.
	maxMessageBytes = 64 << 10
)

// Server é o gRPC interno do Vault. Escuta só em 127.0.0.1 (o host não é
// configurável de propósito: o canal não passa pelo Caddy nem sai da máquina),
// em porta separada da HTTP pública, e toda chamada passa por Authenticate.
// Os valores dos segredos trafegam em claro por este canal, então a restrição
// a loopback é parte da segurança, não só da configuração.
type Server struct {
	grpc     *grpc.Server
	listener net.Listener
}

// NewServer abre o listener em 127.0.0.1:port e cria o servidor gRPC com o
// interceptor de autenticação. Os serviços são registrados depois, pelo Init,
// que chama o InitGRPC de cada service (os handlers moram em grpc_controllers).
// A porta em uso devolve erro aqui, antes de o serviço dizer que subiu. Porta
// "0" escolhe uma livre (usada nos testes).
// Exemplo: port "50052" escuta em 127.0.0.1:50052.
func NewServer(port string, verifier *authn.Verifier, tokens authn.TokenAuthenticator) (*Server, error) {

	listener, err := net.Listen("tcp", net.JoinHostPort(loopbackHost, port))
	if err != nil {
		return nil, fmt.Errorf("grpc: %w", err)
	}

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(Authenticate(verifier, tokens)), grpc.MaxRecvMsgSize(maxMessageBytes))

	return &Server{grpc: server, listener: listener}, nil
}

// Registrar devolve onde os services registram os handlers gRPC (o
// InitGRPC de cada um) antes de o Serve começar.
func (s *Server) Registrar() grpc.ServiceRegistrar {
	return s.grpc
}

// Close fecha o listener sem ter chamado o Serve: é o descarte de um servidor
// que não chegou a subir, quando o registro de algum service falha.
func (s *Server) Close() error {
	return s.listener.Close()
}

func (s *Server) Addr() string {
	return s.listener.Addr().String()
}

func (s *Server) Serve() error {
	return s.grpc.Serve(s.listener)
}

func (s *Server) GracefulStop() {
	s.grpc.GracefulStop()
}

// Shutdown para o gRPC com GracefulStop: recusa chamadas novas e espera as em
// andamento terminarem. Se o ctx vencer antes, corta as que sobraram com Stop,
// para o desligamento nunca passar do prazo. Devolve ctx.Err() nesse caso e nil
// quando tudo terminou a tempo. Exemplo: com uma chamada de 1 s em curso e ctx
// de 5 s, espera a chamada e devolve nil; com ctx de 100 ms, corta e devolve
// context.DeadlineExceeded.
func (s *Server) Shutdown(ctx context.Context) error {

	done := make(chan struct{})
	go func() {
		s.grpc.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		s.grpc.Stop()
		<-done
		return ctx.Err()
	}
}
