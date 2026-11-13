//go:build !windows && !darwin && !linux

package core

import "errors"

func loadWordIterator() (*wordIteratorAPI, error) { return nil, errors.ErrUnsupported }
