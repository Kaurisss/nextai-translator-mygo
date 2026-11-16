//go:build windows || darwin

package desktop

import (
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
	if drag {
		g.lastRelease = time.Time{}
		return true
	}
	doubleClick := !g.lastRelease.IsZero() && now.Sub(g.lastRelease) < 700*time.Millisecond && lastX*lastX+lastY*lastY < 100
	g.last, g.lastRelease = point, now
	return doubleClick
}
