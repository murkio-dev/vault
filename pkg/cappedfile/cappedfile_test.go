package cappedfile_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/murkio-dev/vault/pkg/cappedfile"
	"github.com/stretchr/testify/assert"
)

// Main test

func TestAppend(t *testing.T) {
	t.Run("Quando o arquivo ainda não existe", func(t *testing.T) { whenFileIsMissing(t) })
	t.Run("Quando cabe no limite", func(t *testing.T) { whenFitsTheLimit(t) })
	t.Run("Quando passa do limite", func(t *testing.T) { whenPassesTheLimit(t) })
	t.Run("Quando o arquivo já tem mais linhas que o limite", func(t *testing.T) { whenFileIsAlreadyOver(t) })
	t.Run("Quando a última linha não tem quebra", func(t *testing.T) { whenLastLineHasNoBreak(t) })
	t.Run("Quando o limite é inválido", func(t *testing.T) { whenLimitIsInvalid(t) })
	t.Run("Quando vários escrevem ao mesmo tempo", func(t *testing.T) { whenWritersAreConcurrent(t) })
}

func TestWriter(t *testing.T) {
	t.Run("Quando cria a pasta e escreve", func(t *testing.T) { whenWriterCreatesFolder(t) })
}

// Functions

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	assert.NoError(t, err)
	return string(data)
}

func whenFileIsMissing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.log")

	assert.NoError(t, cappedfile.Append(path, []byte("a"), 3))

	assert.Equal(t, "a\n", read(t, path))
}

func whenFitsTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.log")

	for _, line := range []string{"a\n", "b\n", "c\n"} {
		assert.NoError(t, cappedfile.Append(path, []byte(line), 3))
	}

	assert.Equal(t, "a\nb\nc\n", read(t, path))
}

func whenPassesTheLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.log")

	for _, line := range []string{"a", "b", "c", "d", "e"} {
		assert.NoError(t, cappedfile.Append(path, []byte(line), 3))
	}

	assert.Equal(t, "c\nd\ne\n", read(t, path))
}

func whenFileIsAlreadyOver(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.log")
	assert.NoError(t, os.WriteFile(path, []byte("1\n2\n3\n4\n5\n6\n"), 0o640))

	assert.NoError(t, cappedfile.Append(path, []byte("7"), 2))

	assert.Equal(t, "6\n7\n", read(t, path))
}

func whenLastLineHasNoBreak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.log")
	assert.NoError(t, os.WriteFile(path, []byte("a\nb"), 0o640))

	assert.NoError(t, cappedfile.Append(path, []byte("c"), 10))

	assert.Equal(t, "a\nb\nc\n", read(t, path))
}

func whenLimitIsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.log")

	assert.Error(t, cappedfile.Append(path, []byte("a"), 0))
	_, err := cappedfile.New(path, 0)
	assert.Error(t, err)
}

func whenWritersAreConcurrent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "vault.log")
	const writers, perWriter, limit = 8, 40, 100

	var wg sync.WaitGroup
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				assert.NoError(t, cappedfile.Append(path, []byte(fmt.Sprintf("writer-%d-line-%d", w, i)), limit))
			}
		}()
	}
	wg.Wait()

	lines := strings.Split(strings.TrimSuffix(read(t, path), "\n"), "\n")
	assert.Len(t, lines, limit)
	for _, line := range lines {
		assert.Regexp(t, `^writer-\d-line-\d+$`, line)
	}
}

func whenWriterCreatesFolder(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "vault.log")

	writer, err := cappedfile.New(path, 2)
	assert.NoError(t, err)
	n, err := writer.Write([]byte("one\n"))

	assert.NoError(t, err)
	assert.Equal(t, 4, n)
	assert.Equal(t, "one\n", read(t, path))
}
