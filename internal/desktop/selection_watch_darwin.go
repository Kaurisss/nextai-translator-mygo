package desktop

import (
	"errors"
	"sync"
	"time"

	"github.com/ebitengine/purego/objc"
	"github.com/egoist/mygo"
)

func WatchSelections(onSelection func(Selection), onDismiss func(mygo.Point, bool)) (func(), error) {
	if err := loadMac(); err != nil {
		return nil, err
	}
	var gesture selectionGesture
	var source uintptr
	active := true
	handle := func(event objc.ID) {
		if !active {
			return
		}
		point := mygo.Screen.CursorScreenPoint()
		switch objc.Send[uint64](event, objc.RegisterName("type")) {
		case 1: // NSEventTypeLeftMouseDown
			onDismiss(point, true)
			source = ActiveWindow()
			gesture.press(point)
		case 2: // NSEventTypeLeftMouseUp
			if gesture.release(point, time.Now()) {
				onSelection(Selection{Position: point, point: Point{X: int32(point.X), Y: int32(point.Y)}, window: source})
			}
		case 3, 10: // Right mouse down, key down.
			gesture.armed = false
			onDismiss(point, objc.Send[uint64](event, objc.RegisterName("type")) == 3)
		}
	}
	globalBlock := objc.NewBlock(func(_ objc.Block, event objc.ID) { handle(event) })
	localBlock := objc.NewBlock(func(_ objc.Block, event objc.ID) objc.ID {
		handle(event)
		return event
	})
	var global, local objc.ID
	mygo.RunOnMain(func() {
		events := objc.ID(objc.GetClass("NSEvent"))
		const mask uint64 = 1<<1 | 1<<2 | 1<<3 | 1<<10
		global = events.Send(objc.RegisterName("addGlobalMonitorForEventsMatchingMask:handler:"), mask, globalBlock).Send(objc.RegisterName("retain"))
		local = events.Send(objc.RegisterName("addLocalMonitorForEventsMatchingMask:handler:"), mask, localBlock).Send(objc.RegisterName("retain"))
	})
	stop := sync.OnceFunc(func() {
		mygo.RunOnMain(func() {
			active = false
			for _, monitor := range []objc.ID{global, local} {
				if monitor != 0 {
					objc.ID(objc.GetClass("NSEvent")).Send(objc.RegisterName("removeMonitor:"), monitor)
					monitor.Send(objc.RegisterName("release"))
				}
			}
			globalBlock.Release()
			localBlock.Release()
		})
	})
	if global == 0 || local == 0 {
		stop()
		return nil, errors.New("无法注册 macOS 划词事件监听")
	}
	return stop, nil
}
