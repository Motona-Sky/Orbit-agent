package utils

import (
	"runtime"
)

func GetSystemVersion() string {
	return runtime.GOOS
}
