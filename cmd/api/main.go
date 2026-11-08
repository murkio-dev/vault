package main

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/murkio-dev/vault/internal/entities"
	"github.com/murkio-dev/vault/internal/restful"
	migrations_embed "github.com/murkio-dev/vault/migrations"
	"github.com/murkio-dev/vault/pkg/authn"
	"github.com/murkio-dev/vault/pkg/dotenv"
	"github.com/murkio-dev/vault/pkg/migration"
	"github.com/murkio-dev/vault/pkg/recovery"
	"github.com/murkio-dev/vault/pkg/secretbox"
	"github.com/murkio-dev/vault/pkg/sqlite"
)

func main() {
	mux := http.NewServeMux()
	ctx := context.Background()
	logs := entities.NewLogger(ctx, "./logs.txt")

	fatal := func(err error) {
		logs.FunctionError(err)
		os.Exit(1)
	}

	cfg, err := dotenv.New()
	if err != nil {
		fatal(err)
	}

	db, err := sqlite.New(cfg.Database.GetActivePath())
	if err != nil {
		fatal(err)
	}

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
	if err := srv.ListenAndServe(); err != nil {
		fatal(err)
	}
}
