package app

import (
	"context"
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func TestSelectionToolbarFitsFourActions(t *testing.T) {
	a := testApp(t)
	a.Data.Settings.UIFontSize = 24
	tt := ui.NewTester(a.selectionToolbar, 144, 40)
	var previousRight float32
	for _, label := range []string{"翻译", "润色", "总结", "分析"} {
		bounds, ok := tt.Find(label)
		if !ok || bounds.W < 30 || bounds.H < 30 || bounds.X < previousRight || bounds.X+bounds.W > 144 || bounds.Y < 0 || bounds.Y+bounds.H > 40 {
			t.Fatalf("%s does not fit in the toolbar: %+v", label, bounds)
		}
		previousRight = bounds.X + bounds.W
	}
}

func TestSelectionDismissCancelsOnlySelectionWork(t *testing.T) {
	for _, fromSelection := range []bool{true, false} {
		a := testApp(t)
		a.post = func(fn func()) { fn() }
		a.stopSelectionWatch = func() {}
		a.selectionWatchGeneration = 1
		read, cancelRead := context.WithCancel(context.Background())
		request, cancelRequest := context.WithCancel(context.Background())
		a.selectionCancel, a.cancel = cancelRead, cancelRequest
		a.busy, a.generation, a.inlineGeneration = true, 1, 1
		a.inlineFromSelection = fromSelection
		a.selectedText, a.selectedWindow = "old selection", 1

		a.selectionDismiss(1, mygo.Point{}, true)
		if read.Err() == nil || a.selectedText != "" || a.selectedWindow != 0 {
			t.Fatal("dismissal left the old selection active")
		}
		if (request.Err() != nil) != fromSelection || (a.inlineGeneration == 0) != fromSelection || a.busy == fromSelection {
			t.Fatalf("dismissal affected the wrong request: fromSelection=%v err=%v busy=%v inline=%d", fromSelection, request.Err(), a.busy, a.inlineGeneration)
		}
		cancelRequest()
	}
}
