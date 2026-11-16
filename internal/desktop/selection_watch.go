//go:build windows

package desktop

import (
	"sync"
	"time"

	"github.com/egoist/mygo"
)

type selectionInput struct{ left, right, key bool }

func WatchSelections(onSelection func(Selection), onDismiss func(mygo.Point, bool)) (func(), error) {
	readInput, err := selectionInputReader()
	if err != nil {
		return nil, err
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		var gesture selectionGesture
		var window uintptr
		previous := readInput()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				current := readInput()
				leftChanged := current.left != previous.left
				mousePress := current.left && !previous.left || current.right && !previous.right
				keyPress := current.key && !previous.key
				previous = current
				if !leftChanged && !mousePress && !keyPress {
					continue
				}
				point := mygo.Screen.CursorScreenPoint()
				if mousePress || keyPress {
					onDismiss(point, !keyPress)
				}
				if !leftChanged {
					continue
				}
				physical, err := PointerPosition()
				if current.left {
					window = 0
					if err == nil {
						window = selectionWindowAt(physical)
					}
					gesture.press(point)
				} else if gesture.release(point, now) && err == nil {
					onSelection(Selection{Position: point, point: physical, window: window})
				}
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(stop) }) }, nil
}
