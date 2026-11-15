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
	doubleClick := !g.lastRelease.IsZero() && now.Sub(g.lastRelease) < 700*time.Millisecond && lastX*lastX+lastY*lastY < 100
	g.last, g.lastRelease = point, now
	return drag || doubleClick
}

func WatchSelections(onSelection func(Selection)) (func(), error) {
	pressed, err := mouseButtonReader()
	if err != nil {
		return nil, err
	}
	stop := make(chan struct{})
	go func() {
		ticker := time.NewTicker(20 * time.Millisecond)
		defer ticker.Stop()
		var gesture selectionGesture
		var window uintptr
		down := pressed()
		for {
			select {
			case <-stop:
				return
			case now := <-ticker.C:
				current := pressed()
				if current == down {
					continue
				}
				down = current
				physical, err := PointerPosition()
				if current {
					window = 0
					if err == nil {
						window = selectionWindowAt(physical)
					}
				}
				point := mygo.Screen.CursorScreenPoint()
				if current {
					gesture.press(point)
				} else if gesture.release(point, now) && err == nil {
					go onSelection(Selection{Position: point, point: physical, window: window})
				}
			}
		}
	}()
	var once sync.Once
	return func() { once.Do(func() { close(stop) }) }, nil
}
