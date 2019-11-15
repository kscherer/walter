package util

import (
	"path/filepath"
	"runtime"
)

// https://stackoverflow.com/a/58294680
var (
	_, b, _, _ = runtime.Caller(0)
	Root       = filepath.Join(filepath.Dir(b), "../..")
)
