package core

import "syscall"

var wordLibrary = syscall.NewLazyDLL("icu.dll")

func loadWordIterator() (*wordIteratorAPI, error) {
	return bindWordIterator(func(name string) (uintptr, error) {
		proc := wordLibrary.NewProc(name)
		if err := proc.Find(); err != nil {
			return 0, err
		}
		return proc.Addr(), nil
	})
}
