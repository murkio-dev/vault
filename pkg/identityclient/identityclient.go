// Package identityclient abre a conexão com o gRPC interno do identity. Fala
// sem TLS, porque o canal só existe em loopback: Dial recusa qualquer endereço
// fora dela, então nada do identity atravessa uma rede em claro.
package identityclient

import (
	"errors"
	"net"
	"strconv"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var ErrNotLoopback = errors.New("IDENTITY_GRPC_ADDR must be host:port on loopback (127.0.0.1, ::1 or localhost)")

// ValidateAddr aceita só host:porta com o host em loopback ("localhost" ou um
// IP de loopback) e porta entre 1 e 65535. É o mesmo limite do servidor gRPC
// do identity, que só escuta em 127.0.0.1.
// Exemplo: "127.0.0.1:50051" vale; "10.0.0.5:50051" e "127.0.0.1" não.
func ValidateAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return ErrNotLoopback
	}
	if number, err := strconv.Atoi(port); err != nil || number < 1 || number > 65535 {
		return ErrNotLoopback
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return ErrNotLoopback
	}
	return nil
}

// Dial abre o cliente para addr depois do ValidateAddr. Não conecta de
// verdade: a conexão sobe na primeira chamada, então o identity pode subir
// depois do vault.
func Dial(addr string) (*grpc.ClientConn, error) {
	if err := ValidateAddr(addr); err != nil {
		return nil, err
	}
	return grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
}
