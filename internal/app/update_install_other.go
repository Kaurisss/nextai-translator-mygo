//go:build !windows && !darwin && !linux

package app

import "errors"

// installUpdate reports that this platform has no way to install an update.
func installUpdate(string) error {
	return errors.New("此平台不支持自动更新")
}
