package desktop

import (
	"context"
	"errors"
	"fmt"
	"runtime"

	"github.com/ebitengine/purego/objc"
)

func (s Selection) Read(ctx context.Context, allowInput bool) (string, uintptr, error) {
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	if err := loadMac(); err != nil {
		return "", 0, err
	}
	target := ActiveWindow()
	if target == 0 || s.window != 0 && s.window != target {
		return "", 0, nil
	}
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	pool := objc.ID(objc.GetClass("NSAutoreleasePool")).Send(objc.RegisterName("new"))
	defer pool.Send(objc.RegisterName("drain"))
	app := mac.createApp(int32(target))
	defer mac.release(app)
	mac.setTimeout(app, 0.7)
	system := mac.createSystem()
	defer mac.release(system)
	mac.setTimeout(system, 0.7)
	var focused objc.ID
	status := mac.attribute(system, macString("AXFocusedUIElement"), &focused)
	if err := macSelectionError(status); status != 0 && err != nil {
		return "", 0, err
	}
	var candidates []objc.ID
	if focused != 0 {
		candidates = append(candidates, focused)
		defer mac.release(focused)
	}
	if s.window != 0 && s.point != (Point{}) {
		var element objc.ID
		if mac.elementAt(app, float32(s.point.X), float32(s.point.Y), &element) == 0 {
			for element != 0 {
				candidates = append(candidates, element)
				defer mac.release(element)
				if err := ctx.Err(); err != nil {
					return "", 0, err
				}
				var parent objc.ID
				if mac.attribute(element, macString("AXParent"), &parent) != 0 {
					break
				}
				element = parent
			}
		}
	}
	text := ""
	for _, element := range candidates {
		if err := ctx.Err(); err != nil {
			return "", 0, err
		}
		value, blocked, err := selectedMacText(element, allowInput)
		if blocked || err != nil {
			return "", 0, err
		}
		if text == "" {
			text = value
		}
	}
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}
	if ActiveWindow() != target {
		return "", 0, nil
	}
	return text, target, nil
}

func selectedMacText(element objc.ID, allowInput bool) (text string, blocked bool, err error) {
	for _, attribute := range []string{"AXRole", "AXSubrole"} {
		var value objc.ID
		if mac.attribute(element, macString(attribute), &value) != 0 {
			continue
		}
		role := objc.Send[string](value, objc.RegisterName("UTF8String"))
		mac.release(value)
		if role == "AXSecureTextField" {
			return "", true, nil
		}
		if !allowInput && (role == "AXTextField" || role == "AXTextArea" || role == "AXComboBox") {
			var editable bool
			if mac.settable(element, macString("AXValue"), &editable) != 0 || editable {
				return "", true, nil
			}
		}
	}
	var selected objc.ID
	status := mac.attribute(element, macString("AXSelectedText"), &selected)
	if status == 0 && selected != 0 {
		text = objc.Send[string](selected, objc.RegisterName("UTF8String"))
		mac.release(selected)
		if text != "" {
			return text, false, nil
		}
	} else if err := macSelectionError(status); err != nil {
		return "", false, err
	}
	// WebKit also exposes page selections through text marker ranges.
	var marker objc.ID
	if status := mac.attribute(element, macString("AXSelectedTextMarkerRange"), &marker); status != 0 || marker == 0 {
		return "", false, macSelectionError(status)
	}
	defer mac.release(marker)
	selected = 0
	if status := mac.parameterizedAttribute(element, macString("AXStringForTextMarkerRange"), marker, &selected); status != 0 {
		return "", false, macSelectionError(status)
	}
	if selected == 0 {
		return "", false, nil
	}
	defer mac.release(selected)
	return objc.Send[string](selected, objc.RegisterName("UTF8String")), false, nil
}

func macSelectionError(status int32) error {
	switch status {
	case 0, -25204, -25205, -25208, -25212, -25213: // Success, busy, unsupported, not implemented, no value.
		return nil
	case -25211:
		return errors.New("当前应用的辅助功能权限尚未生效，请在系统设置的辅助功能列表中重新添加当前应用")
	default:
		return fmt.Errorf("辅助功能读取失败（AXError %d）", status)
	}
}
