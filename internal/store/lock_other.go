//go:build !linux

package store

import (
	"fmt"
	"os"
)

func lock(f *os.File) error   { return fmt.Errorf("lit service storage is supported only on Linux") }
func unlock(f *os.File) error { return nil }
