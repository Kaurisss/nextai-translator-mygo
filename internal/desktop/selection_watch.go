//go:build windows || darwin

package desktop

import (
	"sync"
	"time"

	"github.com/egoist/mygo"
)

type selectionGesture struct {
	start, last mygo.Point
	lastRelease time.Time
	armed       bool
}

type selectionInput struct{ left, right, key bool }

func (g *selectionGesture) press(point mygo.Point) {
	g.start = point
	g.armed = true
}

func (g *selectionGesture) release(point mygo.Point, now time.Time) bool {
	if !g.armed {
		return false
	}
	g.armed = false
	dx, dy := point.X-g.start.X, point.Y-g.start.Y
	lastX, lastY := point.X-g.last.X, point.Y-g.last.Y
	drag := dx*dx+dy*dy >= 64
	if drag {
		g.lastRelease = time.Time{}
		return true
	}
	doubleClick := !g.lastRelease.IsZero() && now.Sub(g.lastRelease) < 700*time.Millisecond && lastX*lastX+lastY*lastY < 100
	g.last, g.lastRelease = point, now
	return doubleClick
}

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
