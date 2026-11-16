package desktop

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/ebitengine/purego"
)

var (
	loadMouseState sync.Once
	mouseState     func(int32, uint32) bool
	keyState       func(int32, uint16) bool
	mouseStateErr  error
)

func selectionInputReader() (func() selectionInput, error) {
	loadMouseState.Do(func() {
		var library uintptr
		library, mouseStateErr = purego.Dlopen("/System/Library/Frameworks/CoreGraphics.framework/CoreGraphics", purego.RTLD_NOW)
		if mouseStateErr == nil {
			purego.RegisterLibFunc(&mouseState, library, "CGEventSourceButtonState")
			purego.RegisterLibFunc(&keyState, library, "CGEventSourceKeyState")
		}
	})
	if mouseStateErr != nil {
		return nil, mouseStateErr
	}
	return func() selectionInput {
		return selectionInput{left: mouseState(0, 0), right: mouseState(0, 1), key: keyState(0, 53)} // Escape
	}, nil
}

func selectionWindowAt(Point) uintptr { return 0 }

func (Selection) Read(ctx context.Context, allowInput bool) (string, uintptr, error) {
	script := `tell application "System Events"
set activeApp to first application process whose frontmost is true
set targetPID to unix id of activeApp
set element to value of attribute "AXFocusedUIElement" of activeApp
try
  if subrole of element is "AXSecureTextField" then return ""
end try
if ` + strconv.FormatBool(!allowInput) + ` and (role of element is "AXTextField" or role of element is "AXTextArea") then return ""
try
  set selectedText to value of attribute "AXSelectedText" of element
on error
  return ""
end try
set stillActive to first application process whose frontmost is true
if (unix id of stillActive) is not targetPID or selectedText is missing value then return ""
return (targetPID as text) & linefeed & selectedText
end tell`
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", 0, fmt.Errorf("%s: %w", strings.TrimSpace(string(output)), err)
	}
	pid, text, ok := strings.Cut(strings.TrimSpace(string(output)), "\n")
	if !ok {
		return "", 0, nil
	}
	target, err := strconv.ParseUint(pid, 10, 64)
	return text, uintptr(target), err
}
