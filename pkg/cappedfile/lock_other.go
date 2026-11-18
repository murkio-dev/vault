//go:build !unix

package cappedfile

import "os"

func lock(*os.File) error { return nil }

func unlock(*os.File) {}
