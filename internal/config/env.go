package config

import (
	"bufio"
	"errors"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func IsTest() bool {
	if flag.Lookup("test.v") != nil {
		return true
	}
	return strings.HasSuffix(os.Args[0], ".test")
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
// binário, de onde quer que o binário seja chamado, para o banco (SQLITE_PATH) e o
// log (LOG_PATH) não mudarem de lugar conforme a pasta em que o serviço foi iniciado.
// Vazio, ":memory:" e URIs file: passam como estão. Exemplo: "vault.db" vira
// "/opt/vault/vault.db" quando o binário está em /opt/vault, e "/var/lib/vault.db"
// continua como está.
func ResolvePath(path string) string {
	if path == "" || path == ":memory:" || strings.HasPrefix(path, "file:") || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(BinaryDir(), path)
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
