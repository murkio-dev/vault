package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful"
	"github.com/murkio-dev/vault/internal/rpc"
	migrations_embed "github.com/murkio-dev/vault/migrations"
	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/dotenv"
	"github.com/murkio-dev/vault/pkg/graceful"
	"github.com/murkio-dev/vault/pkg/migration"
	"github.com/murkio-dev/vault/pkg/recovery"
	"github.com/murkio-dev/vault/pkg/secretbox"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

// shutdownTimeout é o prazo do desligamento gracioso. Cobre o WriteTimeout do
// servidor (15 s) com folga: uma requisição que já começou a responder termina
// antes de o servidor ser cortado, e o que passar disso é encerrado à força
// para o deploy nunca ficar preso esperando um cliente lento.
const shutdownTimeout = 20 * time.Second

func main() {
	mux := http.NewServeMux()
	ctx := context.Background()

	cfg, err := dotenv.New()
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

	tokens, err := restful.Init(db, box, cfg.Server.PublicURL, mux)
	if err != nil {
		fatal(err)
	}

	shared, tokenAuthenticator := restful.NewShared(db, box)
	grpcServer, err := rpc.Init(shared, tokenAuthenticator, cfg.Server.GRPCPort)
	if err != nil {
		fatal(err)
	}

	verifier := authn.NewVerifier(cfg.Auth.URL, cfg.Auth.AuthApplicationID())
	authenticated := authn.Authenticate(verifier, tokens, mux)

	logs.Boot(entities.Logs{
		Service: "vault",
		Message: "listening on " + cfg.Server.Port,
	})
	// Timeouts contra Slowloris e teto de corpo, para o caso de o servidor ser
	// alcançado sem o Caddy na frente (infra/Caddyfile). TLS, headers de segurança,
	// rate limit e log de acesso são do Caddy.
	srv := &http.Server{
		Addr:              cfg.Server.Port,
		Handler:           recovery.Recovery(logs, http.MaxBytesHandler(authenticated, 64<<10)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
	logs.Boot(entities.Logs{Service: "vault", Message: "grpc listening on " + grpcServer.Addr()})

	// Em SIGTERM ou Ctrl+C para de aceitar conexões e deixa as requisições em
	// andamento terminarem (até shutdownTimeout) antes de fechar o banco. Sem
	// isso, um deploy cortava a gravação de um segredo no meio, e o fail-closed
	// da auditoria não ajuda quando é o processo que morre.
	if err := graceful.Run(ctx, shutdownTimeout, graceful.HTTP(srv), grpcServer); err != nil {
		logs.FunctionError(err)
		return
	}

	logs.Boot(entities.Logs{Service: "vault", Message: "stopped"})
}
