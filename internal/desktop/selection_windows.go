package desktop

import (
	"context"
	"strconv"
)

var windowFromPhysicalPoint = user32.NewProc("WindowFromPhysicalPoint")
var getAncestor = user32.NewProc("GetAncestor")

func selectionWindowAt(point Point) uintptr {
	packed := uint64(uint32(point.X)) | uint64(uint32(point.Y))<<32
	h, _, _ := windowFromPhysicalPoint.Call(uintptr(packed))
	root, _, _ := getAncestor.Call(h, 2) // GA_ROOT
	return root
}

func mouseButtonReader() (func() bool, error) {
	proc := user32.NewProc("GetAsyncKeyState")
	if err := proc.Find(); err != nil {
		return nil, err
	}
	return func() bool {
		state, _, _ := proc.Call(1)
		return state&0x8000 != 0
	}, nil
}

func (s Selection) Read(ctx context.Context, allowInput bool) (string, uintptr, error) {
	if s.window == 0 || ActiveWindow() != s.window || selectionWindowAt(s.point) != s.window {
		return "", 0, nil
	}
	text, err := powershell(ctx, `$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[Text.Encoding]::UTF8
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName WindowsBase
Add-Type -TypeDefinition 'using System; using System.Runtime.InteropServices; public class SelectionDpi { [DllImport("user32.dll")] public static extern IntPtr SetThreadDpiAwarenessContext(IntPtr context); }'
[void][SelectionDpi]::SetThreadDpiAwarenessContext([IntPtr](-4))
$point=New-Object Windows.Point([int]$env:NEXTAI_POINTER_X,[int]$env:NEXTAI_POINTER_Y)
$element=[Windows.Automation.AutomationElement]::FromPoint($point)
$target=[int64]$env:NEXTAI_SELECTION_WINDOW
$elements=@()
while ($element) {
  if ($element.Current.IsPassword) { exit 0 }
  if ($element.Current.ControlType -eq [Windows.Automation.ControlType]::Edit -and $env:NEXTAI_ALLOW_INPUT -ne 'true') { exit 0 }
  $elements += $element
  if ($element.Current.NativeWindowHandle -eq $target) { break }
  $element=[Windows.Automation.TreeWalker]::RawViewWalker.GetParent($element)
}
if (-not $element) { exit 0 }
foreach ($candidate in $elements) {
  $pattern=$null
  if ($candidate.TryGetCurrentPattern([Windows.Automation.TextPattern]::Pattern,[ref]$pattern) -and $pattern.SupportedTextSelection -ne [Windows.Automation.SupportedTextSelection]::None) {
    $selection=$pattern.GetSelection()
    if ($selection.Count -gt 0 -and $selection[0].CompareEndpoints('Start', $selection[0], 'End') -ne 0) {
      if ($env:NEXTAI_ALLOW_INPUT -ne 'true') {
        $readOnly=$selection[0].GetAttributeValue([Windows.Automation.TextPattern]::IsReadOnlyAttribute)
        if ($readOnly -isnot [bool] -or -not $readOnly) { exit 0 }
      }
      $text=$selection[0].GetText(-1)
      if ($text.Trim()) { [Console]::Write($text); exit 0 }
    }
  }
}`, "", "NEXTAI_POINTER_X="+strconv.Itoa(int(s.point.X)), "NEXTAI_POINTER_Y="+strconv.Itoa(int(s.point.Y)), "NEXTAI_ALLOW_INPUT="+strconv.FormatBool(allowInput), "NEXTAI_SELECTION_WINDOW="+strconv.FormatUint(uint64(s.window), 10))
	if ActiveWindow() != s.window {
		return "", 0, nil
	}
	return text, s.window, err
}
