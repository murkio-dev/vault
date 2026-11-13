package restful_test

import (
	"context"
	"strings"
	"testing"

	"github.com/murkio-dev/vault/internal/restful"
	"github.com/murkio-dev/vault/internal/rpc"
	vaultv1 "github.com/murkio-dev/vault/pkg/rpc/vaultv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// Main test

func TestInternalGRPC(t *testing.T) {
	t.Run("Quando escuta.", func(t *testing.T) { whenGRPCListensOnLoopback(t) })
	t.Run("Quando o gRPC lê o ambiente com o token do SDK.", func(t *testing.T) { whenGRPCReadsEnvironment(t) })
	t.Run("Quando a credencial não é um token do SDK válido.", func(t *testing.T) { whenGRPCCredentialIsInvalid(t) })
	t.Run("Quando o uri_id não bate.", func(t *testing.T) { whenGRPCURIDoesNotMatch(t) })
	t.Run("Quando o token é revogado.", func(t *testing.T) { whenGRPCTokenIsRevoked(t) })
}

// Helpers

func (s *stack) grpcClient() (vaultv1.VaultServiceClient, string) {
	s.t.Helper()

	shared, tokens := restful.NewShared(s.db, s.box)
	server, err := rpc.Init(shared, tokens, "0")
	require.NoError(s.t, err)
	go func() { _ = server.Serve() }()
	s.t.Cleanup(server.GracefulStop)

	conn, err := grpc.NewClient(server.Addr(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(s.t, err)
	s.t.Cleanup(func() { _ = conn.Close() })

	return vaultv1.NewVaultServiceClient(conn), server.Addr()
}

func bearer(token string) context.Context {
	return metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
}

// Functions

func whenGRPCListensOnLoopback(t *testing.T) {
	_, addr := newStack(t).grpcClient()

	assert.True(t, strings.HasPrefix(addr, "127.0.0.1:"), "o gRPC interno escuta só em loopback")
}

func whenGRPCReadsEnvironment(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	adminMember := s.memberID(&maria, app)
	s.createSecret(&maria, app, adminMember, "DATABASE_URL", "string", "valor-secreto")
	s.createSecret(&maria, app, adminMember, "API_KEY", "api_key", "sk-123")
	token := s.newToken(&maria, app, "me", "ci")
	client, _ := s.grpcClient()

	all, err := client.ReadEnv(bearer(token.token), &vaultv1.ReadEnvRequest{UriId: token.uri})
	filtered, filteredErr := client.ReadEnv(bearer(token.token), &vaultv1.ReadEnvRequest{UriId: token.uri, Names: []string{"API_KEY"}})

	require.NoError(t, err)
	assert.Equal(t, token.uri, all.UriId)
	values := map[string]string{}
	for _, variable := range all.Variables {
		values[variable.Name] = variable.Value
	}
	assert.Equal(t, map[string]string{"DATABASE_URL": "valor-secreto", "API_KEY": "sk-123"}, values)
	require.NoError(t, filteredErr)
	require.Len(t, filtered.Variables, 1)
	assert.Equal(t, "API_KEY", filtered.Variables[0].Name)
	assert.Equal(t, 2, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'env.read' AND token_id = ?`, token.id), "cada leitura deixa rastro")
}

func whenGRPCCredentialIsInvalid(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	token := s.newToken(&maria, app, "me", "ci")
	client, _ := s.grpcClient()
	request := &vaultv1.ReadEnvRequest{UriId: token.uri}

	_, anonymous := client.ReadEnv(context.Background(), request)
	_, garbage := client.ReadEnv(bearer("vt_"+strings.Repeat("A", 43)), request)
	_, person := client.ReadEnv(bearer(maria.token), request)
	_, wrongScheme := client.ReadEnv(metadata.AppendToOutgoingContext(context.Background(), "authorization", "Basic "+token.token), request)

	assert.Equal(t, codes.Unauthenticated, status.Code(anonymous))
	assert.Equal(t, codes.Unauthenticated, status.Code(garbage))
	assert.Equal(t, codes.Unauthenticated, status.Code(person), "o JWT de pessoa não lê o runtime")
	assert.Equal(t, codes.Unauthenticated, status.Code(wrongScheme))
}

func whenGRPCURIDoesNotMatch(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	dev, _ := s.join(&maria, app, "developer")
	adminToken := s.newToken(&maria, app, "me", "a")
	devToken := s.newToken(&dev, app, "me", "d")
	client, _ := s.grpcClient()

	_, crossed := client.ReadEnv(bearer(devToken.token), &vaultv1.ReadEnvRequest{UriId: adminToken.uri})

	assert.Equal(t, codes.NotFound, status.Code(crossed), "o token do dev não abre a URI do admin")
	assert.Equal(t, 1, s.count(`SELECT COUNT(*) FROM audit_logs WHERE action = 'access.denied' AND token_id = ?`, devToken.id))
}

func whenGRPCTokenIsRevoked(t *testing.T) {
	s := newStack(t)
	maria := s.person()
	app := s.createApplication(&maria, "minha-app")
	token := s.newToken(&maria, app, "me", "ci")
	client, _ := s.grpcClient()
	_, err := client.ReadEnv(bearer(token.token), &vaultv1.ReadEnvRequest{UriId: token.uri})
	require.NoError(t, err)

	_, execErr := s.db.Exec(`UPDATE access_tokens SET revoked_at = CURRENT_TIMESTAMP WHERE id = ?`, token.id)
	require.NoError(t, execErr)
	_, afterRevoke := client.ReadEnv(bearer(token.token), &vaultv1.ReadEnvRequest{UriId: token.uri})

	assert.Equal(t, codes.Unauthenticated, status.Code(afterRevoke), "token revogado perde acesso na hora")
}
