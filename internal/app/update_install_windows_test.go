//go:build windows

package app

import (
	"encoding/base64"
	"encoding/binary"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestWindowsUpdateScriptNSIS(t *testing.T) {
	script := windowsUpdateScript(
		4321,
		`C:\Users\张三\AppData\Local\Temp\nextai-update-1\nextai-translator-native-setup-0.3.0-amd64.exe`,
		`C:\Users\张三\AppData\Local\Programs\NextAI Translator Native`,
		`C:\Users\张三\AppData\Local\Programs\NextAI Translator Native\NextAI Translator Native.exe`,
		`C:\Users\张三\AppData\Local\Temp\nextai-update-1`,
		false,
	)
	for _, want := range []string{
		"$targetPid = 4321",
		`'/S /D=' + $installDir`,
		`'C:\Users\张三\AppData\Local\Programs\NextAI Translator Native'`,
		`Start-Process -FilePath $exe`,
		`Remove-Item -LiteralPath $workDir -Recurse -Force`,
	} {
		if !strings.Contains(script, want) {
			t.Errorf("script missing %q:\n%s", want, script)
		}
	}
}

func TestWindowsUpdateScriptInno(t *testing.T) {
	script := windowsUpdateScript(
		99,
		`C:\Temp\setup-x64.exe`,
		`C:\Apps\O'Brien & Co\NextAI`,
		`C:\Apps\O'Brien & Co\NextAI\NextAI Translator Native.exe`,
		`C:\Temp`,
		true,
	)
	if !strings.Contains(script, `'/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /DIR="' + $installDir + '"'`) {
		t.Fatalf("missing Inno silent args:\n%s", script)
	}
	if !strings.Contains(script, `'C:\Apps\O''Brien & Co\NextAI'`) {
		t.Fatalf("single quote not escaped:\n%s", script)
	}
}

func TestEncodePowerShellCommandRoundTrip(t *testing.T) {
	original := "$p = 'C:\\Users\\张三\\NextAI Translator Native.exe'"
	encoded := encodePowerShellCommand(original)
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	if len(raw)%2 != 0 {
		t.Fatalf("UTF-16LE byte length must be even, got %d", len(raw))
	}
	units := make([]uint16, len(raw)/2)
	for i := range units {
		units[i] = binary.LittleEndian.Uint16(raw[i*2:])
	}
	got := string(utf16.Decode(units))
	if got != original {
		t.Fatalf("decoded = %q, want %q", got, original)
	}
}

func TestIsInnoSetup(t *testing.T) {
	dir := t.TempDir()
	innoPath := filepath.Join(dir, "inno.exe")
	nsisPath := filepath.Join(dir, "nsis.exe")
	if err := os.WriteFile(innoPath, []byte("MZ...Inno Setup Setup Data (6.3.0)..."), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(nsisPath, []byte("MZ...NullsoftInst..."), 0o600); err != nil {
		t.Fatal(err)
	}
	if !isInnoSetup(innoPath) {
		t.Fatal("expected Inno Setup installer to be detected")
	}
	if isInnoSetup(nsisPath) {
		t.Fatal("NSIS installer should not be detected as Inno Setup")
	}
}
