package app

import (
	"errors"
	"fmt"

	"github.com/egoist/mygo"
)

type credentialStore interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
	Delete(string) error
}

var errCredentialsUnavailable = errors.New("系统凭据库不可用")

func writeCredentials(store credentialStore, keys map[string]string) error {
	for name, key := range keys {
		var err error
		if key == "" {
			err = store.Delete(name)
		} else {
			err = store.Set(name, []byte(key))
		}
		if err != nil && !errors.Is(err, mygo.ErrSecretNotFound) {
			return fmt.Errorf("写入 %s 凭据: %w", name, err)
		}
	}
	return nil
}

func updateCredentials(store credentialStore, keys map[string]string) (map[string]string, error) {
	previous := make(map[string]string, len(keys))
	for name := range keys {
		key, err := store.Get(name)
		if errors.Is(err, errors.ErrUnsupported) {
			return nil, errCredentialsUnavailable
		}
		if err != nil && !errors.Is(err, mygo.ErrSecretNotFound) {
			return nil, fmt.Errorf("读取 %s 凭据: %w", name, err)
		}
		previous[name] = string(key)
	}
	if err := writeCredentials(store, keys); err != nil {
		return nil, errors.Join(err, writeCredentials(store, previous))
	}
	return previous, nil
}
