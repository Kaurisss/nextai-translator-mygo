package desktop

import (
	"context"
	"fmt"
	"runtime"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var windowFromPhysicalPoint = user32.NewProc("WindowFromPhysicalPoint")
var getAncestor = user32.NewProc("GetAncestor")
var selectionDPI = user32.NewProc("SetThreadDpiAwarenessContext")
var openInputDesktop = user32.NewProc("OpenInputDesktop")
var getThreadDesktop = user32.NewProc("GetThreadDesktop")
var setThreadDesktop = user32.NewProc("SetThreadDesktop")
var closeDesktop = user32.NewProc("CloseDesktop")
var currentThreadID = syscall.NewLazyDLL("kernel32.dll").NewProc("GetCurrentThreadId")

// The thread's inherited desktop may differ from the input desktop at login.
// Callers pin their goroutine until restore has run.
func attachInputDesktop() (restore func(), err error) {
	thread, _, _ := currentThreadID.Call()
	previous, _, _ := getThreadDesktop.Call(thread)
	input, _, err := openInputDesktop.Call(0, 0, 0x02000000) // MAXIMUM_ALLOWED
	if input == 0 {
		return nil, err
	}
	if ok, _, err := setThreadDesktop.Call(input); ok == 0 {
		closeDesktop.Call(input)
		return nil, err
	}
	return func() {
		setThreadDesktop.Call(previous)
		closeDesktop.Call(input)
	}, nil
}

func packedPoint(point Point) uintptr {
	return uintptr(uint64(uint32(point.X)) | uint64(uint32(point.Y))<<32)
}

func selectionWindowAt(point Point) uintptr {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	restore, err := attachInputDesktop()
	if err != nil {
		return 0
	}
	defer restore()
	return selectionWindowAtOnThread(point)
}

func selectionWindowAtOnThread(point Point) uintptr {
	if previous, _, _ := selectionDPI.Call(^uintptr(3)); previous != 0 {
		defer selectionDPI.Call(previous)
	}
	h, _, _ := windowFromPhysicalPoint.Call(packedPoint(point))
	root, _, _ := getAncestor.Call(h, 2) // GA_ROOT
	return root
}

func selectionInputReader() (func() selectionInput, error) {
	proc := user32.NewProc("GetAsyncKeyState")
	if err := proc.Find(); err != nil {
		return nil, err
	}
	pressed := func(key uintptr) bool {
		state, _, _ := proc.Call(key)
		return state&0x8000 != 0
	}
	return func() selectionInput {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		restore, err := attachInputDesktop()
		if err != nil {
			return selectionInput{}
		}
		defer restore()
		return selectionInput{left: pressed(1), right: pressed(2), key: pressed(0x1b) || // Escape
			pressed(0x21) || pressed(0x22) || pressed(0x23) || pressed(0x24) || // Page Up/Down, End, Home
			pressed(0x25) || pressed(0x26) || pressed(0x27) || pressed(0x28)} // arrows
	}, nil
}

var (
	uiaOle32            = syscall.NewLazyDLL("ole32.dll")
	uiaOleAut32         = syscall.NewLazyDLL("oleaut32.dll")
	uiaInitialize       = uiaOle32.NewProc("CoInitializeEx")
	uiaUninitialize     = uiaOle32.NewProc("CoUninitialize")
	uiaCreateInstance   = uiaOle32.NewProc("CoCreateInstance")
	uiaVariantClear     = uiaOleAut32.NewProc("VariantClear")
	uiaStringLen        = uiaOleAut32.NewProc("SysStringLen")
	uiaFreeString       = uiaOleAut32.NewProc("SysFreeString")
	clsidCUIAutomation8 = syscall.GUID{Data1: 0xe22ad333, Data2: 0xb25f, Data3: 0x460c, Data4: [8]byte{0x83, 0xd0, 0x05, 0x81, 0x10, 0x73, 0x95, 0xc9}}
	iidUIAutomation2    = syscall.GUID{Data1: 0x34723aff, Data2: 0x0c9d, Data3: 0x49d0, Data4: [8]byte{0x98, 0x96, 0x7a, 0xb5, 0x2d, 0xf8, 0xcd, 0x8a}}
	iidUITextPattern    = syscall.GUID{Data1: 0x32eba289, Data2: 0x3583, Data3: 0x42c9, Data4: [8]byte{0x9c, 0x59, 0x3b, 0x6d, 0x9a, 0x1e, 0x9b, 0x6a}}
)

// Vtable slots from the Windows SDK UIAutomationClient.h, including IUnknown.
const (
	uiaElementFromPoint = 7
	uiaRawViewWalker    = 16
	uiaConnectionLimit  = 61
	uiaTransactionLimit = 63
	uiaGetParent        = 3
	uiaGetPatternAs     = 14
	uiaControlType      = 21
	uiaIsPassword       = 35
	uiaNativeWindow     = 36
	uiaGetSelection     = 5
	uiaSupportsSelect   = 8
	uiaArrayLength      = 3
	uiaArrayElement     = 4
	uiaCompareEndpoints = 5
	uiaGetAttribute     = 9
	uiaGetText          = 12
)

type uiaObject struct{ vtable *[64]uintptr }

//go:uintptrescapes
func (obj *uiaObject) call(slot int, args ...uintptr) bool {
	r, _, _ := syscall.SyscallN(obj.vtable[slot], append([]uintptr{uintptr(unsafe.Pointer(obj))}, args...)...)
	return int32(r) >= 0
}

func (s Selection) Read(ctx context.Context, allowInput bool) (string, uintptr, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	restore, err := attachInputDesktop()
	if err != nil {
		return "", 0, nil
	}
	defer restore()
	if s.window == 0 || ActiveWindow() != s.window || selectionWindowAtOnThread(s.point) != s.window {
		return "", 0, nil
	}
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	hr, _, _ := uiaInitialize.Call(0, 0) // COINIT_MULTITHREADED
	if int32(hr) < 0 {
		return "", 0, fmt.Errorf("CoInitializeEx: 0x%08x", uint32(hr))
	}
	defer uiaUninitialize.Call()
	var automation *uiaObject
	hr, _, _ = uiaCreateInstance.Call(uintptr(unsafe.Pointer(&clsidCUIAutomation8)), 0, 1,
		uintptr(unsafe.Pointer(&iidUIAutomation2)), uintptr(unsafe.Pointer(&automation)))
	if int32(hr) < 0 || automation == nil {
		return "", 0, fmt.Errorf("CoCreateInstance: 0x%08x", uint32(hr))
	}
	defer automation.call(2)
	automation.call(uiaConnectionLimit, 700)
	automation.call(uiaTransactionLimit, 700)
	if previous, _, _ := selectionDPI.Call(^uintptr(3)); previous != 0 {
		defer selectionDPI.Call(previous)
	}
	text := s.readText(ctx, automation, allowInput)
	if ctx.Err() != nil || text == "" || ActiveWindow() != s.window {
		return "", 0, nil
	}
	return text, s.window, nil
}

func (s Selection) readText(ctx context.Context, automation *uiaObject, allowInput bool) string {
	var walker, element *uiaObject
	if !automation.call(uiaRawViewWalker, uintptr(unsafe.Pointer(&walker))) || walker == nil {
		return ""
	}
	defer walker.call(2)
	if !automation.call(uiaElementFromPoint, packedPoint(s.point), uintptr(unsafe.Pointer(&element))) {
		return ""
	}
	var candidates []*uiaObject
	for element != nil {
		defer element.call(2)
		var password, controlType int32
		var window uintptr
		if ctx.Err() != nil || !element.call(uiaIsPassword, uintptr(unsafe.Pointer(&password))) || password != 0 ||
			(!allowInput && (!element.call(uiaControlType, uintptr(unsafe.Pointer(&controlType))) || controlType == 50004)) || // UIA_EditControlTypeId
			!element.call(uiaNativeWindow, uintptr(unsafe.Pointer(&window))) {
			return ""
		}
		candidates = append(candidates, element)
		if window == s.window {
			for _, candidate := range candidates {
				if ctx.Err() != nil || ActiveWindow() != s.window {
					return ""
				}
				if text, blocked := selectedUIAText(candidate, allowInput); blocked || text != "" {
					return text
				}
			}
			return ""
		}
		var parent *uiaObject
		if !walker.call(uiaGetParent, uintptr(unsafe.Pointer(element)), uintptr(unsafe.Pointer(&parent))) {
			return ""
		}
		element = parent
	}
	return ""
}

func selectedUIAText(element *uiaObject, allowInput bool) (text string, blocked bool) {
	var pattern, ranges, selection *uiaObject
	var supported, count, endpoints int32
	if !element.call(uiaGetPatternAs, 10014, uintptr(unsafe.Pointer(&iidUITextPattern)), uintptr(unsafe.Pointer(&pattern))) || pattern == nil {
		return "", false
	}
	defer pattern.call(2)
	if !pattern.call(uiaSupportsSelect, uintptr(unsafe.Pointer(&supported))) || supported == 0 ||
		!pattern.call(uiaGetSelection, uintptr(unsafe.Pointer(&ranges))) || ranges == nil {
		return "", false
	}
	defer ranges.call(2)
	if !ranges.call(uiaArrayLength, uintptr(unsafe.Pointer(&count))) || count == 0 ||
		!ranges.call(uiaArrayElement, 0, uintptr(unsafe.Pointer(&selection))) || selection == nil {
		return "", false
	}
	defer selection.call(2)
	if !selection.call(uiaCompareEndpoints, 0, uintptr(unsafe.Pointer(selection)), 1, uintptr(unsafe.Pointer(&endpoints))) || endpoints == 0 {
		return "", false
	}
	if !allowInput {
		var attr struct {
			kind uint16
			_    [3]uint16
			data [2]uintptr
		}
		ok := selection.call(uiaGetAttribute, 40015, uintptr(unsafe.Pointer(&attr))) // UIA_IsReadOnlyAttributeId
		readOnly := ok && attr.kind == 11 && int16(attr.data[0]) == -1               // VT_BOOL, VARIANT_TRUE
		uiaVariantClear.Call(uintptr(unsafe.Pointer(&attr)))
		if !readOnly {
			return "", true
		}
	}
	var value *uint16
	if !selection.call(uiaGetText, ^uintptr(0), uintptr(unsafe.Pointer(&value))) || value == nil {
		return "", false
	}
	defer uiaFreeString.Call(uintptr(unsafe.Pointer(value)))
	n, _, _ := uiaStringLen.Call(uintptr(unsafe.Pointer(value)))
	return string(utf16.Decode(unsafe.Slice(value, int(n)))), false
}
