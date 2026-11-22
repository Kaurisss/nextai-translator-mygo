//go:build windows

package app

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"

	"github.com/egoist/mygo"
)

const (
	createNewProcessGroup = 0x00000200
	createNoWindow        = 0x08000000
)

// installUpdate starts a detached PowerShell helper that waits for this
// process to exit, runs the downloaded installer silently into the current
// install directory, restarts the app, and cleans up the download directory.
func installUpdate(path string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	installDir := filepath.Dir(exe)
	probe, err := os.MkdirTemp(installDir, ".nextai-update-")
	if err != nil {
		return fmt.Errorf("无法写入应用所在目录: %w", err)
	}
	os.Remove(probe)

	inno := isInnoSetup(path)
	script := windowsUpdateScript(os.Getpid(), path, installDir, exe, filepath.Dir(path), inno)
	encoded := encodePowerShellCommand(script)

	cmd := exec.Command(windowsPowerShellPath(), "-NoProfile", "-NonInteractive", "-WindowStyle", "Hidden", "-EncodedCommand", encoded)
	// The helper outlives the app: it waits for it to exit, runs the
	// installer silently, starts the new version, and removes the temp folder.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: createNewProcessGroup | createNoWindow,
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	_ = cmd.Process.Release()
	mygo.App.Quit()
	return nil
}

func windowsPowerShellPath() string {
	if root := os.Getenv("SystemRoot"); root != "" {
		p := filepath.Join(root, "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "powershell.exe"
}

// isInnoSetup reports whether the installer executable was built with Inno
// Setup rather than NSIS by scanning its uncompressed stub header.
func isInnoSetup(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	buf := make([]byte, 512<<10)
	n, _ := io.ReadFull(f, buf)
	return bytes.Contains(buf[:n], []byte("Inno Setup"))
}

// psSingleQuote quotes s as a PowerShell single-quoted literal.
func psSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// encodePowerShellCommand encodes a PowerShell script as UTF-16LE base64 for
// powershell.exe -EncodedCommand, avoiding OEM codepage issues on non-ASCII paths.
func encodePowerShellCommand(script string) string {
	units := utf16.Encode([]rune(script))
	raw := make([]byte, len(units)*2)
	for i, u := range units {
		binary.LittleEndian.PutUint16(raw[i*2:], u)
	}
	return base64.StdEncoding.EncodeToString(raw)
}

// windowsUpdateScript returns the PowerShell script that waits for pid to exit,
// installs the update into installDir, starts exe again, and removes workDir
// when installation succeeds.
func windowsUpdateScript(pid int, installer, installDir, exe, workDir string, inno bool) string {
	installDir = strings.TrimRight(installDir, `\`)
	if len(installDir) == 2 && installDir[1] == ':' {
		installDir += `\`
	}
	argExpr := `'/S /D=' + $installDir`
	if inno {
		argExpr = `'/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /DIR="' + $installDir + '"'`
	}
	fallbackExe := filepath.Join(installDir, "NextAI Translator Native.exe")
	return strings.Join([]string{
		"$ErrorActionPreference = 'SilentlyContinue'",
		"$targetPid = " + strconv.Itoa(pid),
		"$installer = " + psSingleQuote(installer),
		"$installDir = " + psSingleQuote(installDir),
		"$exe = " + psSingleQuote(exe),
		"$fallbackExe = " + psSingleQuote(fallbackExe),
		"$workDir = " + psSingleQuote(workDir),
		"try {",
		"  $proc = Get-Process -Id $targetPid -ErrorAction Stop",
		"  [void]$proc.WaitForExit(30000)",
		"} catch {}",
		"Start-Sleep -Milliseconds 300",
		"$ok = $false",
		"try {",
		"  $p = Start-Process -FilePath $installer -ArgumentList (" + argExpr + ") -Wait -PassThru",
		"  if ($p -and $p.ExitCode -eq 0) { $ok = $true }",
		"} catch {}",
		"if (Test-Path -LiteralPath $exe) {",
		"  Start-Process -FilePath $exe",
		"} elseif (Test-Path -LiteralPath $fallbackExe) {",
		"  Start-Process -FilePath $fallbackExe",
		"}",
		"if ($ok) {",
		"  for ($i = 0; $i -lt 5; $i++) {",
		"    Remove-Item -LiteralPath $workDir -Recurse -Force -ErrorAction SilentlyContinue",
		"    if (-not (Test-Path -LiteralPath $workDir)) { break }",
		"    Start-Sleep -Milliseconds 200",
		"  }",
		"}",
	}, "\n")
}
