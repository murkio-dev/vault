package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/murkio-dev/vault/internal/config"
	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful"
	migrations_embed "github.com/murkio-dev/vault/migrations"
	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/dependencies"
	"github.com/murkio-dev/vault/pkg/graceful"
	"github.com/murkio-dev/vault/pkg/httpx"
	"github.com/murkio-dev/vault/pkg/identityclient"
	"github.com/murkio-dev/vault/pkg/migration"
	identityv1 "github.com/murkio-dev/vault/pkg/rpc/identityv1"
	"github.com/murkio-dev/vault/pkg/secretbox"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

func main() {
	mux := http.NewServeMux()
	ctx := context.Background()

	cfg, err := config.New()
	if err != nil {
		fmt.Fprintln(os.Stderr, "vault: config:", err)
		os.Exit(1)
	}

	logs := entities.NewLogger(ctx, cfg.Log.Path)

	fatal := func(err error) {
		logs.FunctionError(err)
		os.Exit(1)
	}

	db, err := sqlite.New(cfg.Database.GetActivePath())
	if err != nil {
		fatal(err)
	}
	defer db.Close()

	// A chave de cifragem é conferida já no boot: com ela inválida o serviço não
	// sobe, em vez de falhar só na primeira gravação de um segredo.
	box, err := secretbox.FromBase64(cfg.Security.EncryptionKey, cfg.Security.PreviousKeys())
	if err != nil {
		fatal(err)
	}

	migrator := migration.New(db, migrations_embed.FS)
	if err := migrator.Run(ctx, "sqlite"); err != nil {
		fatal(err)
	}

	tokens, err := restful.Init(db, box, cfg.Server.BackendURL, mux)
	if err != nil {
		fatal(err)
	}

	identity, err := identityclient.Dial(cfg.Auth.GRPCAddr)
	if err != nil {
		fatal(err)
	}
	defer identity.Close()
	verifier := authn.NewVerifier(identityv1.NewSigningKeyServiceClient(identity), cfg.Auth.AuthApplicationID())

	deps, tokenAuthenticator := dependencies.New(db, box)
	grpcServer, err := restful.InitGRPC(db, deps, verifier, tokenAuthenticator, cfg.Server.BackendURL, cfg.Server.GRPCPort)
	if err != nil {
		fatal(err)
	}

	httpServer, err := httpx.NewServer(httpx.ServerInput{
		Service: "vault",
		Port:    cfg.Server.Port,
		Logs:    logs,
		Handler: authn.Authenticate(verifier, tokens, mux),
	})
	if err != nil {
		fatal(err)
	}
	logs.Boot(entities.Logs{Service: "vault", Message: "grpc listening on " + grpcServer.Addr()})

	// Em SIGTERM ou Ctrl+C para de aceitar conexões e deixa as requisições em
	// andamento terminarem (até httpx.ShutdownTimeout) antes de fechar o banco. Sem
	// isso, um deploy cortava a gravação de um segredo no meio, e o fail-closed
	// da auditoria não ajuda quando é o processo que morre.
	if err := graceful.Run(ctx, httpx.ShutdownTimeout, httpServer, grpcServer); err != nil {
		logs.FunctionError(err)
		return
	}

	logs.Boot(entities.Logs{Service: "vault", Message: "stopped"})
}
