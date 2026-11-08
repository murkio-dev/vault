// Utilize as tags
// `environmentRef` para referenciar o .env
// `required` para keys obrigatorias
// `default` para fallback

package dotenv

import (
	"bufio"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"reflect"
	"strings"
	"uuid"
)

type Config struct {
	Server   Server
	Database Database
	Auth     Auth
	Security Security
}

type Server struct {
	Environment string `environmentRef:"ENVIRONMENT" required:"true"`
	Port        string `environmentRef:"SERVER_PORT" required:"true"`
	PublicURL   string `environmentRef:"PUBLIC_URL"`
}

type Database struct {
	SQLitePath     string `environmentRef:"SQLITE_PATH" required:"true"`
	SQLiteTestPath string `environmentRef:"SQLITE_TEST_PATH" required:"false"`
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
	_ = loadEnvFile(".env")

	cfg := &Config{}
	bindConfig(reflect.ValueOf(cfg).Elem())

	if err := validateRequired(reflect.ValueOf(cfg).Elem()); err != nil {
		return nil, err
	}

	if err := cfg.Auth.validate(); err != nil {
		return nil, err
	}

	return cfg, nil
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
