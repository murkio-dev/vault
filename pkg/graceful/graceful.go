package graceful

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Server é o que o Run sabe subir e parar. Serve bloqueia até o servidor parar;
// Shutdown para de aceitar conexões novas, deixa as em andamento terminarem e
// volta quando acabarem ou quando o ctx vencer.
type Server interface {
	Serve() error
	Shutdown(ctx context.Context) error
}

type httpServer struct{ srv *http.Server }

// HTTP adapta um http.Server para o Run. O Serve devolve nil quando o servidor
// foi parado por Shutdown (http.ErrServerClosed não é falha), e o erro de
// qualquer outra parada. Exemplo: HTTP(&http.Server{Addr: ":8080", Handler: h}).
func HTTP(srv *http.Server) Server {
	return httpServer{srv: srv}
}

func (h httpServer) Serve() error {
	if err := h.srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (h httpServer) Shutdown(ctx context.Context) error {
	return h.srv.Shutdown(ctx)
}

// Signals são os sinais que pedem o desligamento. SIGTERM é o do deploy e do
// supervisor (rc.d no FreeBSD, systemd) e os.Interrupt é o Ctrl+C. SIGKILL não
// entra: o sistema não deixa o processo tratá-lo, então não há como ser gracioso.
// A lista é variável de pacote para os testes poderem trocá-la, se precisarem.
var Signals = []os.Signal{syscall.SIGTERM, os.Interrupt}

// Run sobe todos os servidores e só volta depois de pará-los. Para quando
// chega um dos Signals (ou o ctx pai é cancelado) ou quando algum servidor cai
// sozinho; nos dois casos pede Shutdown a todos ao mesmo tempo, dando ao
// conjunto até timeout. Passado o prazo, o Shutdown de cada um recebe o ctx já
// vencido e deve forçar o fim. Devolve o erro do servidor que caiu, junto com
// os erros do desligamento; um desligamento limpo por sinal devolve nil. Quem
// chama fecha o banco e o cache só depois do Run voltar, com as requisições em
// andamento já terminadas. Exemplo: Run(ctx, 20*time.Second, HTTP(srv), grpc).
func Run(ctx context.Context, timeout time.Duration, servers ...Server) error {

	ctx, stop := signal.NotifyContext(ctx, Signals...)
	defer stop()

	failures := make(chan error, len(servers))
	for _, server := range servers {
		go func() { failures <- server.Serve() }()
	}

	var serveErr error
	remaining := len(servers)
	select {
	case <-ctx.Done():
	case serveErr = <-failures:
		remaining--
	}

	stop()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	shutdownErrs := make(chan error, len(servers))
	for _, server := range servers {
		go func() { shutdownErrs <- server.Shutdown(shutdownCtx) }()
	}

	errs := []error{serveErr}
	for range servers {
		errs = append(errs, <-shutdownErrs)
	}
	for range remaining {
		if err := <-failures; err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}
