package config

import (
	"fmt"
	"os"
	"reflect"
	"strconv"
	"strings"
	"uuid"

	"github.com/murkio-dev/vault/pkg/identityclient"
)

type Config struct {
	Server   Server
	Log      Log
	Database Database
	Auth     Auth
	Security Security
}

type Server struct {
	Port       string `environmentRef:"APPLICATION_PORT" required:"true"`
	BackendURL string `environmentRef:"APPLICATION_BACKEND"`
	GRPCPort   string `environmentRef:"GRPC_PORT" default:"50052"`
}

type Database struct {
	SQLitePath     string `environmentRef:"SQLITE_PATH" default:"vault.db"`
	SQLiteTestPath string `environmentRef:"SQLITE_TEST_PATH" required:"false"`
}

type Log struct {
	Path string `environmentRef:"LOG_PATH" default:"vault.log"`
}

type Auth struct {
	// GRPCAddr é o gRPC interno do identity (só loopback), de onde vem o JWKS
	// que verifica o JWT das pessoas. Toda comunicação interna é gRPC: o vault
	// nunca chama o identity pela API REST.
	GRPCAddr      string `environmentRef:"IDENTITY_GRPC_ADDR" default:"127.0.0.1:50051"`
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
	cfg.Log.Path = ResolvePath(cfg.Log.Path)

	if err := cfg.Auth.validate(); err != nil {
		return nil, err
	}

	if err := cfg.Server.validateGRPCPort(); err != nil {
		return nil, err
	}

	return cfg, nil
}

func (s Server) validateGRPCPort() error {
	if port, err := strconv.Atoi(s.Port); err != nil || port < 1 || port > 65535 {
		return fmt.Errorf("APPLICATION_PORT must be a port number between 1 and 65535")
	}

	port, err := strconv.Atoi(s.GRPCPort)
	if err != nil || port < 1 || port > 65535 || s.Port == s.GRPCPort {
		return fmt.Errorf("GRPC_PORT must be a port number between 1 and 65535, different from APPLICATION_PORT")
	}

	return nil
}

func (a Auth) validate() error {
	if err := identityclient.ValidateAddr(a.GRPCAddr); err != nil {
		return err
	}

	id, err := uuid.Parse(a.ApplicationID)
	if err != nil || id == (uuid.UUID{}) {
		return fmt.Errorf("AUTH_APPLICATION_ID must be a non-zero UUID")
	}
	return nil
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
			if envVal, exists := os.LookupEnv(envKey); exists && strings.TrimSpace(envVal) != "" {
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
