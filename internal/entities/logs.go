package entities

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

type Logs struct {
	Timestamp string `json:"@timestamp"`
	Level     string `json:"log.level,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	Type      string `json:"log.type,omitempty"` // boot | function_error

	Origin string `json:"error.origin,omitempty"`
	Error  string `json:"error.message,omitempty"`

	Service      string          `json:"service.name,omitempty"`
	Version      string          `json:"service.version,omitempty"`
	Environment  string          `json:"service.environment,omitempty"`
	Message      string          `json:"message,omitempty"`
	GoVersion    string          `json:"service.runtime.version,omitempty"`
	PID          int             `json:"process.pid,omitempty"`
	Dependencies map[string]bool `json:"service.dependencies,omitempty"`
}

const DefaultLogPath = "logs.txt"

const (
	TypeBoot          = "boot"
	TypeFunctionError = "function_error"
)

const modulePrefix = "github.com/murkio-dev/vault/"

type Domain struct {
	Path    string
	context context.Context
	mu      *sync.Mutex
}

func NewLogger(ctx context.Context, path string) *Domain {
	if path == "" {
		path = DefaultLogPath
	}
	return &Domain{
		Path:    path,
		context: ctx,
		mu:      &sync.Mutex{},
	}
}

func (d Domain) Write(l Logs) error {
	if d.context != nil {
		if err := d.context.Err(); err != nil {
			return err
		}
	}
	if l.Timestamp == "" {
		l.Timestamp = time.Now().UTC().Format(time.RFC3339Nano)
	}
	if l.Level == "" {
		l.Level = "info"
	}
	if l.PID == 0 {
		l.PID = os.Getpid()
	}
	if l.GoVersion == "" {
		l.GoVersion = runtime.Version()
	}

	line, err := json.Marshal(l)
	if err != nil {
		return err
	}
	line = append(line, '\n')

	if d.mu != nil {
		d.mu.Lock()
		defer d.mu.Unlock()
	}

	f, err := os.OpenFile(d.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.Write(line)
	return err
}

func (d Domain) Boot(l Logs) error {
	l.Type = TypeBoot
	if l.Message == "" {
		l.Message = "service started"
	}
	return d.Write(l)
}

func (d Domain) FunctionError(err error) error {
	return d.functionError(2, err)
}

func (d Domain) FunctionErrorAt(origin string, err error) error {
	if err == nil {
		return nil
	}
	return d.Write(Logs{
		Type:   TypeFunctionError,
		Level:  "error",
		Origin: origin,
		Error:  err.Error(),
	})
}

func (d Domain) functionError(skip int, err error) error {
	if err == nil {
		return nil
	}
	return d.FunctionErrorAt(callerOrigin(skip+1), err)
}

// callerOrigin monta o campo error.origin do log a partir da pilha de chamadas.
// Recebe skip, o número de frames a subir (como em runtime.Caller) para chegar
// na função que originou o erro, e devolve "<pasta>/<arquivo>.go/<função>".
// O prefixo do módulo é removido e o receiver "(*T)" vira "T".
// Exemplo: um erro em (*Runner).Run de pkg/migration/migration.go vira
// "pkg/migration/migration.go/Runner.Run". Devolve "unknown" se a pilha
// não puder ser resolvida.
func callerOrigin(skip int) string {
	pc, file, _, ok := runtime.Caller(skip)
	if !ok {
		return "unknown"
	}
	name := "unknown"
	dir := ""
	if fn := runtime.FuncForPC(pc); fn != nil {
		full := strings.TrimPrefix(fn.Name(), modulePrefix)
		slash := strings.LastIndex(full, "/")
		dot := strings.Index(full[slash+1:], ".")
		if dot >= 0 {
			dir = full[:slash+1+dot]
			name = full[slash+1+dot+1:]
		}
	}

	name = strings.NewReplacer("(*", "", ")", "").Replace(name)
	if dir != "" {
		dir += "/"
	}
	return dir + filepath.Base(file) + "/" + name
}

func (d Domain) log(level, msg string) error {
	return d.Write(Logs{Level: level, Message: msg})
}

func (d Domain) Info(msg string) error  { return d.log("info", msg) }
func (d Domain) Warn(msg string) error  { return d.log("warn", msg) }
func (d Domain) Error(msg string) error { return d.log("error", msg) }
func (d Domain) Debug(msg string) error { return d.log("debug", msg) }
