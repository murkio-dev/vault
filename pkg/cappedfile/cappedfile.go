package cappedfile

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type Writer struct {
	path     string
	maxLines int
}

func New(path string, maxLines int) (*Writer, error) {
	if maxLines < 1 {
		return nil, fmt.Errorf("cappedfile: maxLines must be at least 1")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, fmt.Errorf("cappedfile: creating the directory: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return nil, fmt.Errorf("cappedfile: opening %s: %w", path, err)
	}
	return &Writer{path: path, maxLines: maxLines}, file.Close()
}

// Write guarda p como uma linha do arquivo. Cada chamada de Write é uma linha
// inteira (é como o slog e o log do vault escrevem), então não há buffer entre
// chamadas. Devolve len(p) quando a linha entrou no arquivo.
func (w *Writer) Write(p []byte) (int, error) {
	if err := Append(w.path, p, w.maxLines); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Append acrescenta line ao fim do arquivo em path e deixa só as últimas
// maxLines linhas nele. Existe para o log de dev e de produção caber num
// arquivo de tamanho limitado, sem rotação nem processo de limpeza.
// Entradas: line sem "\n" no fim ganha um; maxLines menor que 1 é erro; o
// arquivo é criado se faltar. Vários processos podem escrever no mesmo path ao
// mesmo tempo: o arquivo é aberto e travado (flock) a cada chamada, então
// ninguém escreve num arquivo já substituído, e linhas nunca se misturam.
// Exemplo: com maxLines 3 e o arquivo "a\nb\nc\n", Append("d") deixa "b\nc\nd\n".
// Sem corte só o fim é escrito; com corte o arquivo é reescrito de uma vez
// (o conteúdo novo entra antes de o excesso ser truncado).
func Append(path string, line []byte, maxLines int) error {
	if maxLines < 1 {
		return fmt.Errorf("cappedfile: maxLines must be at least 1")
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o640)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := lock(file); err != nil {
		return err
	}
	defer unlock(file)

	current, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	size := len(current)

	if size > 0 && current[size-1] != '\n' {
		current = append(current, '\n')
	}
	current = append(current, line...)
	if current[len(current)-1] != '\n' {
		current = append(current, '\n')
	}

	kept := lastLines(current, maxLines)
	if len(kept) == len(current) {
		_, err = file.WriteAt(current[size:], int64(size))
		return err
	}

	if _, err := file.WriteAt(kept, 0); err != nil {
		return err
	}
	return file.Truncate(int64(len(kept)))
}

func lastLines(data []byte, maxLines int) []byte {
	if bytes.Count(data, []byte{'\n'}) <= maxLines {
		return data
	}
	end := len(data)
	for kept := 0; kept < maxLines; kept++ {
		end = bytes.LastIndexByte(data[:end-1], '\n') + 1
	}
	return data[end:]
}
