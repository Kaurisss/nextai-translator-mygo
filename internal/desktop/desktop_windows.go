package desktop

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
	"unicode/utf16"
	"unsafe"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
	"github.com/egoist/mygo"
)

var user32 = syscall.NewLazyDLL("user32.dll")
var foreground = user32.NewProc("GetForegroundWindow")
var setForeground = user32.NewProc("SetForegroundWindow")
var keyEvent = user32.NewProc("keybd_event")
var clipboardSequence = user32.NewProc("GetClipboardSequenceNumber")

var backgroundBlurSupported = sync.OnceValue(func() bool {
	var version struct {
		size, major, minor, build, platform uint32
		servicePack                         [128]uint16
	}
	version.size = uint32(unsafe.Sizeof(version))
	status, _, _ := syscall.NewLazyDLL("ntdll.dll").NewProc("RtlGetVersion").Call(uintptr(unsafe.Pointer(&version)))
	return status == 0 && (version.major > 10 || version.major == 10 && version.build >= 22621)
})

func SupportsBackgroundBlur() bool { return backgroundBlurSupported() }

func ActiveWindow() uintptr { h, _, _ := foreground.Call(); return h }

func PointerPosition() (Point, error) {
	var point Point
	ok, _, _ := user32.NewProc("GetPhysicalCursorPos").Call(uintptr(unsafe.Pointer(&point)))
	if ok == 0 {
		return Point{}, errors.New("无法读取指针位置")
	}
	return point, nil
}

func hideCommand(cmd *exec.Cmd) { cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true} }

func chord(key uintptr) {
	keyEvent.Call(0x11, 0, 0, 0)
	keyEvent.Call(key, 0, 0, 0)
	keyEvent.Call(key, 0, 2, 0)
	keyEvent.Call(0x11, 0, 2, 0)
}

func CopySelection(ctx context.Context) error {
	before, _, _ := clipboardSequence.Call()
	// A global shortcut may still have Shift held when its callback runs.
	for _, key := range []uintptr{0x10, 0x11, 0x12} {
		keyEvent.Call(key, 0, 2, 0)
	}
	chord(0x43)
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return errors.New("未取得选中文本，请先选择文本后重试")
		case <-ticker.C:
			now, _, _ := clipboardSequence.Call()
			if now != before {
				return nil
			}
		}
	}
}

func Paste(ctx context.Context, target uintptr) error {
	if target == 0 {
		return errors.New("请先使用全局快捷键从目标窗口取词")
	}
	ok, _, _ := setForeground.Call(target)
	if ok == 0 {
		return errors.New("无法激活原输入窗口")
	}
	timer := time.NewTimer(120 * time.Millisecond)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
	}
	if ActiveWindow() != target {
		return errors.New("原输入窗口未获得焦点，请重新取词后重试")
	}
	chord(0x56)
	return nil
}

func powershell(ctx context.Context, script, input string, env ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "powershell.exe", "-NoLogo", "-NoProfile", "-NonInteractive", "-STA", "-Command", "[Console]::OutputEncoding=[Text.Encoding]::UTF8\n"+script)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Stdin = strings.NewReader(input)
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("系统操作失败: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimPrefix(strings.TrimSpace(string(out)), "\ufeff"), nil
}

func Speak(ctx context.Context, text string, options core.SpeechSettings, voice string) error {
	rate := int(math.Round(10 * math.Log2(float64(options.Rate))))
	_, err := powershell(ctx, `$ErrorActionPreference='Stop'; [Console]::InputEncoding=[Text.Encoding]::UTF8; Add-Type -AssemblyName System.Speech; $s=New-Object System.Speech.Synthesis.SpeechSynthesizer; try { $s.Rate=[int]$env:NEXTAI_SPEECH_RATE; $s.Volume=[int]$env:NEXTAI_SPEECH_VOLUME; if ($env:NEXTAI_SPEECH_VOICE) { $s.SelectVoice($env:NEXTAI_SPEECH_VOICE) }; $s.Speak([Console]::In.ReadToEnd()) } finally { $s.Dispose() }`, text, "NEXTAI_SPEECH_RATE="+strconv.Itoa(max(-10, min(10, rate))), "NEXTAI_SPEECH_VOLUME="+strconv.Itoa(int(options.Volume)), "NEXTAI_SPEECH_VOICE="+voice)
	return err
}

func SystemVoices(ctx context.Context) ([]Voice, error) {
	out, err := powershell(ctx, `$ErrorActionPreference='Stop'; [Console]::OutputEncoding=[Text.Encoding]::UTF8; Add-Type -AssemblyName System.Speech; $s=New-Object System.Speech.Synthesis.SpeechSynthesizer; try { $voices=@($s.GetInstalledVoices() | Where-Object Enabled | ForEach-Object { @{Name=$_.VoiceInfo.Name; Language=$_.VoiceInfo.Culture.Name} }); ConvertTo-Json -InputObject $voices -Compress } finally { $s.Dispose() }`, "")
	if err != nil {
		return nil, err
	}
	var voices []Voice
	err = json.Unmarshal([]byte(out), &voices)
	return voices, err
}

func CopyAll(ctx context.Context) error {
	for _, key := range []uintptr{0x10, 0x11, 0x12} {
		keyEvent.Call(key, 0, 2, 0)
	}
	chord(0x41)
	return CopySelection(ctx)
}

func WriteText(ctx context.Context, target uintptr, text, newline string) error {
	if newline == "" {
		mygo.Clipboard.WriteText(text)
		return Paste(ctx, target)
	}
	mods, key, err := ParseChord(newline)
	if err != nil {
		return err
	}
	for i, line := range strings.Split(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ActiveWindow() != target {
			return errors.New("写作目标窗口已切换，已停止写入")
		}
		if i > 0 {
			for _, mod := range mods {
				keyEvent.Call(mod, 0, 0, 0)
			}
			keyEvent.Call(key, 0, 0, 0)
			keyEvent.Call(key, 0, 2, 0)
			for j := len(mods) - 1; j >= 0; j-- {
				keyEvent.Call(mods[j], 0, 2, 0)
			}
		}
		if line != "" {
			type input struct {
				Type        uint32
				Padding     uint32
				VK, Scan    uint16
				Flags, Time uint32
				Extra       uintptr
				Tail        [8]byte
			}
			var events []input
			for _, unit := range utf16.Encode([]rune(line)) {
				events = append(events, input{Type: 1, Scan: unit, Flags: 4}, input{Type: 1, Scan: unit, Flags: 6})
			}
			written, _, err := user32.NewProc("SendInput").Call(uintptr(len(events)), uintptr(unsafe.Pointer(&events[0])), unsafe.Sizeof(events[0]))
			if written != uintptr(len(events)) {
				return fmt.Errorf("写入文本失败: SendInput %d/%d: %v", written, len(events), err)
			}
		}
	}
	return nil
}

func QuickText(ctx context.Context, position Point) (string, error) {
	return powershell(ctx, `$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[Text.Encoding]::UTF8
Add-Type -AssemblyName UIAutomationClient
Add-Type -AssemblyName UIAutomationTypes
Add-Type -AssemblyName WindowsBase
Add-Type -TypeDefinition 'using System; using System.Text; using System.Runtime.InteropServices; public class Pointer {
[StructLayout(LayoutKind.Sequential)] public struct Point { public int X; public int Y; }
[DllImport("user32.dll")] public static extern IntPtr SetThreadDpiAwarenessContext(IntPtr context);
[DllImport("user32.dll")] public static extern IntPtr WindowFromPoint(Point p);
[DllImport("user32.dll",CharSet=CharSet.Unicode)] public static extern int GetClassName(IntPtr h,StringBuilder s,int n);
[DllImport("user32.dll")] public static extern int GetWindowLong(IntPtr h,int index);
[DllImport("user32.dll",EntryPoint="SendMessageTimeoutW")] public static extern IntPtr Message(IntPtr h,uint m,UIntPtr w,IntPtr l,uint flags,uint timeout,out UIntPtr result);
[DllImport("user32.dll",EntryPoint="SendMessageTimeoutW",CharSet=CharSet.Unicode)] public static extern IntPtr ReadText(IntPtr h,uint m,UIntPtr w,StringBuilder l,uint flags,uint timeout,out UIntPtr result);
}'
[void][Pointer]::SetThreadDpiAwarenessContext([IntPtr](-4))
$p=New-Object Pointer+Point; $p.X=[int]$env:NEXTAI_POINTER_X; $p.Y=[int]$env:NEXTAI_POINTER_Y
$native=[Pointer]::WindowFromPoint($p)
$class=New-Object Text.StringBuilder(256); [void][Pointer]::GetClassName($native,$class,256)
$isEdit=$class.ToString() -match '(^Edit$|RichEdit|WindowsForms\d*\.EDIT)'
if ($isEdit -and (([Pointer]::GetWindowLong($native,-16) -band 0x20) -ne 0)) { throw '密码输入框不支持快捷取词' }
if ($isEdit) {
  $length=[UIntPtr]::Zero
  if ([Pointer]::Message($native,14,[UIntPtr]::Zero,[IntPtr]::Zero,2,500,[ref]$length) -eq [IntPtr]::Zero) { throw '读取文本控件超时' }
  $count=$length.ToUInt64()
  if ($count -gt 1000000) { throw '文本过长，请选中需要翻译的部分' }
  $text=New-Object Text.StringBuilder([int]$count+1)
  $copied=[UIntPtr]::Zero
  if ([Pointer]::ReadText($native,13,[UIntPtr]::new($count+1),$text,2,500,[ref]$copied) -eq [IntPtr]::Zero) { throw '读取文本控件超时' }
  if ($text.ToString().Trim()) { [Console]::Write($text.ToString()); exit 0 }
}
$point=New-Object Windows.Point($p.X,$p.Y)
$element=[Windows.Automation.AutomationElement]::FromPoint($point)
for ($i=0; $element -and $i -lt 10; $i++) {
  if ($element.Current.IsPassword) { throw '密码输入框不支持快捷取词' }
  $pattern=$null
  if ($element.TryGetCurrentPattern([Windows.Automation.TextPattern]::Pattern,[ref]$pattern)) {
    $selection=$pattern.GetSelection()
    if ($selection.Count -gt 0) { $selected=$selection[0].GetText(-1); if ($selected.Trim()) { [Console]::Write($selected); exit 0 } }
    $range=$pattern.RangeFromPoint($point)
    $range.ExpandToEnclosingUnit([Windows.Automation.Text.TextUnit]::Word)
    $word=$range.GetText(-1)
    if ($word.Trim()) { [Console]::Write($word); exit 0 }
  }
  $valuePattern=$null
  if ($element.TryGetCurrentPattern([Windows.Automation.ValuePattern]::Pattern,[ref]$valuePattern)) {
    $value=$valuePattern.Current.Value
    if ($value.Trim()) { [Console]::Write($value); exit 0 }
  }
  if ($element.Current.ControlType -eq [Windows.Automation.ControlType]::Text -and $element.Current.Name.Trim()) {
    [Console]::Write($element.Current.Name); exit 0
  }
  $element=[Windows.Automation.TreeWalker]::ControlViewWalker.GetParent($element)
}
throw '未读取到指针位置的文字，请选择文本后重试'`, "", "NEXTAI_POINTER_X="+strconv.Itoa(int(position.X)), "NEXTAI_POINTER_Y="+strconv.Itoa(int(position.Y)))
}

//go:embed ocr.ps1
var ocrScript string

func OCR(ctx context.Context, path string) (string, error) {
	out, err := powershell(ctx, ocrScript, "", "NEXTAI_OCR_PATH="+path)
	if err != nil {
		return "", err
	}
	var results []string
	if err := json.Unmarshal([]byte(out), &results); err != nil {
		return "", fmt.Errorf("读取 OCR 结果: %w", err)
	}
	if len(results) == 0 {
		return "", errors.New("OCR 未返回识别结果")
	}
	return bestOCRResult(results), nil
}
