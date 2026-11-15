//go:build windows || darwin

package desktop

import (
	"testing"
	"time"

	"github.com/egoist/mygo"
)

func TestSelectionGesture(t *testing.T) {
	var gesture selectionGesture
	now := time.Now()
	gesture.press(mygo.Point{X: 10, Y: 10})
	if !gesture.release(mygo.Point{X: 30, Y: 10}, now) {
		t.Fatal("drag was not recognized")
	}
	gesture.press(mygo.Point{X: 100, Y: 100})
	if gesture.release(mygo.Point{X: 100, Y: 100}, now.Add(time.Second)) {
		t.Fatal("single click recognized as selection")
	}
	gesture.press(mygo.Point{X: 101, Y: 100})
	if !gesture.release(mygo.Point{X: 101, Y: 100}, now.Add(1200*time.Millisecond)) {
		t.Fatal("double click was not recognized")
	}
}
