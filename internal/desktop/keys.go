package desktop

import (
	"fmt"
	"strconv"
	"strings"
)

type Point struct{ X, Y int32 }

func ParseChord(chord string) ([]uintptr, uintptr, error) {
	var modifiers []uintptr
	parts := strings.Split(chord, "+")
	for _, mod := range parts[:len(parts)-1] {
		key := map[string]uintptr{"ctrl": 0x11, "control": 0x11, "shift": 0x10, "alt": 0x12, "super": 0x5b}[strings.ToLower(mod)]
		if key == 0 {
			return nil, 0, fmt.Errorf("不支持的换行按键: %s", chord)
		}
		modifiers = append(modifiers, key)
	}
	last := strings.ToUpper(parts[len(parts)-1])
	key := map[string]uintptr{"ENTER": 13, "RETURN": 13, "TAB": 9, "SPACE": 32, "BACKSPACE": 8, "DELETE": 46, "LEFT": 37, "UP": 38, "RIGHT": 39, "DOWN": 40, "HOME": 36, "END": 35, "PAGEUP": 33, "PAGEDOWN": 34, "INSERT": 45}[last]
	if len(last) == 1 && (last[0] >= 'A' && last[0] <= 'Z' || last[0] >= '0' && last[0] <= '9') {
		key = uintptr(last[0])
	}
	if strings.HasPrefix(last, "F") {
		if n, err := strconv.Atoi(last[1:]); err == nil && n >= 1 && n <= 24 {
			key = uintptr(111 + n)
		}
	}
	if key == 0 {
		return nil, 0, fmt.Errorf("不支持的换行按键: %s", chord)
	}
	return modifiers, key, nil
}
