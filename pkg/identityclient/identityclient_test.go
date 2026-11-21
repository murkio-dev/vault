package identityclient_test

import (
	"testing"

	"github.com/murkio-dev/vault/pkg/identityclient"
	"github.com/stretchr/testify/assert"
)

func TestDial(t *testing.T) {
	t.Run("Quando o endereço é loopback.", func(t *testing.T) { whenAddrIsLoopback(t) })
	t.Run("Quando o endereço sai da máquina ou é inválido.", func(t *testing.T) { whenAddrIsRejected(t) })
}

func whenAddrIsLoopback(t *testing.T) {
	for _, addr := range []string{"127.0.0.1:50051", "localhost:50051", "[::1]:50051"} {
		conn, err := identityclient.Dial(addr)
		if assert.NoError(t, err, addr) {
			assert.NoError(t, conn.Close())
		}
	}
}

func whenAddrIsRejected(t *testing.T) {
	for _, addr := range []string{"", "127.0.0.1", "10.0.0.5:50051", "auth.example.com:443", "http://127.0.0.1:50051", "127.0.0.1:0", "127.0.0.1:99999"} {
		conn, err := identityclient.Dial(addr)
		assert.ErrorIs(t, err, identityclient.ErrNotLoopback, addr)
		assert.Nil(t, conn)
	}
}
