//go:build darwin || linux

package core

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/ebitengine/purego"
)

func loadWordIterator() (*wordIteratorAPI, error) {
	paths := []string{"/usr/lib/libicucore.A.dylib"}
	if runtime.GOOS == "linux" {
		paths = nil
		for _, pattern := range []string{"/usr/lib/*/libicuuc.so.*", "/lib/*/libicuuc.so.*", "/usr/lib64/libicuuc.so.*", "/usr/lib/libicuuc.so.*", "/lib64/libicuuc.so.*"} {
			matches, err := filepath.Glob(pattern)
			if err != nil {
				return nil, err
			}
			paths = append(paths, matches...)
		}
	}
	for _, path := range paths {
		handle, err := purego.Dlopen(path, purego.RTLD_NOW|purego.RTLD_LOCAL)
		if err != nil {
			continue
		}
		suffix := ""
		if _, version, ok := strings.Cut(filepath.Base(path), ".so."); ok {
			suffix = "_" + strings.Split(version, ".")[0]
		}
		api, err := bindWordIterator(func(name string) (uintptr, error) { return purego.Dlsym(handle, name+suffix) })
		if err == nil {
			return api, nil
		}
		purego.Dlclose(handle)
	}
	return nil, fmt.Errorf("ICU word segmentation library is unavailable on %s", runtime.GOOS)
}
