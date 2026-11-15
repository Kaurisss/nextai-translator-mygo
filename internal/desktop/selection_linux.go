package desktop

import (
	"context"
	"errors"
)

func WatchSelections(func(Selection)) (func(), error) {
	return nil, errors.New("当前系统不支持全局鼠标划词")
}

func (Selection) Read(context.Context, bool) (string, uintptr, error) {
	return "", 0, errors.New("当前系统不支持全局鼠标划词")
}
