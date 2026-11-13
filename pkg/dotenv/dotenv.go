// Utilize as tags
// `environmentRef` para referenciar o .env
// `required` para keys obrigatorias
// `default` para fallback

package dotenv

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"uuid"
)

type Config struct {
	Server   Server
	Log      Log
	Database Database
	Auth     Auth
	Security Security
}

type Server struct {
	Environment string `environmentRef:"ENVIRONMENT" required:"true"`
	Port        string `environmentRef:"SERVER_PORT" required:"true"`
	PublicURL   string `environmentRef:"PUBLIC_URL"`
	GRPCPort    string `environmentRef:"GRPC_PORT" default:"50052"`
}

type Database struct {
	SQLitePath     string `environmentRef:"SQLITE_PATH" required:"true"`
	SQLiteTestPath string `environmentRef:"SQLITE_TEST_PATH" required:"false"`
}

type Log struct {
	Path string `environmentRef:"LOG_PATH"`
}

type Auth struct {
	URL           string `environmentRef:"AUTH_URL" required:"true"`
	ApplicationID string `environmentRef:"AUTH_APPLICATION_ID" required:"true"`
}

type Security struct {
	EncryptionKey          string `environmentRef:"ENCRYPTION_KEY" required:"true"`
	EncryptionKeysPrevious string `environmentRef:"ENCRYPTION_KEYS_PREVIOUS"`
}

func (s Security) PreviousKeys() []string {
	var keys []string
	for _, key := range strings.Split(s.EncryptionKeysPrevious, ",") {
		if key = strings.TrimSpace(key); key != "" {
			keys = append(keys, key)
		}
	}
	return keys
}

func (a Auth) AuthApplicationID() uuid.UUID {
	id, _ := uuid.Parse(a.ApplicationID)
	return id
}

func New() (*Config, error) {
	if err := Load(); err != nil {
		return nil, err
	}

	cfg := &Config{}
	bindConfig(reflect.ValueOf(cfg).Elem())

	if err := validateRequired(reflect.ValueOf(cfg).Elem()); err != nil {
		return nil, err
	}

	cfg.Database.SQLitePath = ResolvePath(cfg.Database.SQLitePath)
	cfg.Log.Path = LogPath(cfg.Log.Path)

	if err := cfg.Auth.validate(); err != nil {
		return nil, err
	}

	if err := cfg.Server.validateGRPCPort(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (s Server) validateGRPCPort() error {

	port, err := strconv.Atoi(s.GRPCPort)
	if err != nil || port < 1 || port > 65535 || strings.TrimPrefix(s.Port, ":") == s.GRPCPort {
		return fmt.Errorf("GRPC_PORT must be a port number between 1 and 65535, different from SERVER_PORT")
	}

	return nil
}

func (a Auth) validate() error {
	if err := validateAuthURL(a.URL); err != nil {
		return err
	}

	id, err := uuid.Parse(a.ApplicationID)
	if err != nil || id == (uuid.UUID{}) {
		return fmt.Errorf("AUTH_APPLICATION_ID must be a non-zero UUID")
	}
	return nil
}

// validateAuthURL aceita uma URL https:// (http:// só em localhost ou loopback,
// para dev), sem caminho extra, credenciais, query ou fragmento. O JWKS sai
// dela: uma chave de outra origem aceitaria token forjado, e uma URL com
// credenciais as espalharia em log. A URL nunca é montada com dado do token.
// Exemplo: https://auth.example.com vale; http://auth.example.com e
// https://user:pass@auth.example.com não.
func validateAuthURL(raw string) error {
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return fmt.Errorf("AUTH_URL must be an https url without credentials, query or fragment (http only on localhost)")
	}

	switch u.Scheme {
	case "https":
		return nil
	case "http":
		host := u.Hostname()
		if host == "localhost" {
			return nil
		}
		if ip := net.ParseIP(host); ip != nil && ip.IsLoopback() {
			return nil
		}
	}

	return fmt.Errorf("AUTH_URL must be an https url without credentials, query or fragment (http only on localhost)")
}

func IsTest() bool {
	if flag.Lookup("test.v") != nil {
		return true
	}
	return strings.HasSuffix(os.Args[0], ".test")
}

func (d *Database) GetActivePath() string {
	if IsTest() && d.SQLiteTestPath != "" {
		return d.SQLiteTestPath
	}
	return d.SQLitePath
}

func bindConfig(v reflect.Value) {
	t := v.Type()

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		val := v.Field(i)

		if val.Kind() == reflect.Struct {
			bindConfig(val)
			continue
		}

		if val.Kind() != reflect.String {
			continue
		}

		envKey := field.Tag.Get("environmentRef")
		defaultVal := field.Tag.Get("default")

		finalVal := defaultVal
		if envKey != "" {
			if envVal, exists := os.LookupEnv(envKey); exists {
				finalVal = envVal
			}
		}

		val.SetString(finalVal)
	}
}

func validateRequired(v reflect.Value) error {
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		val := v.Field(i)

		if val.Kind() == reflect.Struct {
			if err := validateRequired(val); err != nil {
				return err
			}
			continue
		}

		isRequired := field.Tag.Get("required") == "true"
		if isRequired && val.Kind() == reflect.String && val.String() == "" {
			return fmt.Errorf("configuração obrigatória ausente para o campo: %s", field.Name)
		}
	}
	return nil
}

func loadEnvFile(filename string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)

			if _, exists := os.LookupEnv(key); !exists {
				os.Setenv(key, val)
			}
		}
	}

	return scanner.Err()
}

// Load lê os arquivos .env para o ambiente do processo, sem sobrescrever o que
// já está definido: primeiro o do lado do binário, depois o da pasta atual. O
// ambiente do processo vale mais que os dois. Arquivo ausente não é erro; um
// que existe e não abre (permissão, por exemplo) é, para a config não subir
// pela metade sem o operador saber. Exemplo: com bin/.env e ./.env, a variável
// que está só no segundo ainda entra, e a que está nos dois fica com o valor do
// primeiro.
func Load() error {

	var errs []error
	seen := map[string]bool{}

	for _, dir := range []string{BinaryDir(), workingDir()} {
		path := filepath.Join(dir, ".env")
		if seen[path] {
			continue
		}
		seen[path] = true

		if err := loadEnvFile(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// ResolvePath devolve path absoluto: relativo, ele é relativo à pasta do
// binário, de onde quer que o binário seja chamado, para o banco e o log não
// mudarem de lugar conforme a pasta em que o serviço foi iniciado. Vazio,
// ":memory:" e URIs file: passam como estão. Exemplo: "vault.db" vira
// "/opt/vault/vault.db" quando o binário está em /opt/vault.
func ResolvePath(path string) string {
	if path == "" || path == ":memory:" || strings.HasPrefix(path, "file:") || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(BinaryDir(), path)
}

// LogPath é o arquivo de log: LOG_PATH (relativo ao binário) ou
// <nome do binário>.log ao lado dele. Dois processos na mesma pasta escrevem
// cada um no próprio arquivo. Exemplo: vault-api vira vault-api.log.
func LogPath(configured string) string {
	if configured == "" {
		name := strings.TrimSuffix(filepath.Base(executable()), filepath.Ext(executable()))
		configured = name + ".log"
	}
	return ResolvePath(configured)
}

// BinaryDir é a pasta do executável. No `go run` e nos testes o executável
// fica numa pasta temporária do build, e a pasta atual é usada no lugar dela:
// o banco de dev não some a cada execução.
func BinaryDir() string {
	exe := executable()
	dir := filepath.Dir(exe)
	if exe == "" || strings.Contains(dir, "go-build") || IsTest() {
		return workingDir()
	}
	return dir
}

func executable() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		return resolved
	}
	return exe
}

func workingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	return dir
}
