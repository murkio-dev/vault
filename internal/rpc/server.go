package rpc

import (
	"context"
	"fmt"
	"net"

	"github.com/murkio-dev/vault/pkg/authn"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"google.golang.org/grpc"
)

const loopbackHost = "127.0.0.1"

// Server é o gRPC interno do Vault. Escuta só em 127.0.0.1 (o host não é
// configurável de propósito: o canal não passa pelo Caddy nem sai da máquina),
// em porta separada da HTTP pública, e toda chamada passa por Authenticate.
// Os valores dos segredos trafegam em claro por este canal, então a restrição
// a loopback é parte da segurança, não só da configuração.
type Server struct {
	grpc     *grpc.Server
	listener net.Listener
}

// NewServer abre o listener em 127.0.0.1:port e registra o VaultService atrás
// do interceptor de autenticação. A porta em uso devolve erro aqui, antes de o
// serviço dizer que subiu. Porta "0" escolhe uma livre (usada nos testes).
// Exemplo: port "50052" escuta em 127.0.0.1:50052.
func NewServer(port string, tokens authn.TokenAuthenticator, controller *VaultController) (*Server, error) {

	listener, err := net.Listen("tcp", net.JoinHostPort(loopbackHost, port))
	if err != nil {
		return nil, fmt.Errorf("grpc: %w", err)
	}

	server := grpc.NewServer(grpc.ChainUnaryInterceptor(Authenticate(tokens)))
	vaultv1.RegisterVaultServiceServer(server, controller)

	return &Server{grpc: server, listener: listener}, nil
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
