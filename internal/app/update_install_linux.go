//go:build linux

package app

import (
	"archive/tar"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/egoist/mygo"
)

// installUpdate unpacks the archive into a staging folder next to the running
// executable, swaps the files into place, and relaunches the app.
func installUpdate(path string) error {
	if os.Getenv("APPIMAGE") != "" {
		return errors.New("AppImage 版本不支持自动更新，请手动替换安装包")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if real, err := filepath.EvalSymlinks(exe); err == nil {
		exe = real
	}
	dir := filepath.Dir(exe)
	work, err := os.MkdirTemp(dir, ".nextai-update-")
	if err != nil {
		return errors.New("当前版本由系统包管理器安装或所在目录不可写，请使用包管理器或手动更新")
	}
	defer os.RemoveAll(work)

	stage := filepath.Join(work, "extracted")
	if err := os.Mkdir(stage, 0o755); err != nil {
		return err
	}
	if err := extractUpdate(path, stage); err != nil {
		return err
	}
	if err := swapUpdateDir(stage, dir); err != nil {
		return err
	}
	mygo.App.Relaunch()
	return nil
}

// extractUpdate unpacks the .tar.gz at path into dir after validating entry
// paths and symlink targets.
func extractUpdate(path, dir string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	count := 0
	for {
		header, err := tr.Next()
		if err == io.EOF {
			if count == 0 {
				return errors.New("更新归档为空")
			}
			return nil
		}
		if err != nil {
			return err
		}
		name := filepath.Clean(filepath.FromSlash(header.Name))
		if name == "." {
			continue
		}
		if filepath.IsAbs(name) || name == ".." || strings.HasPrefix(name, ".."+string(filepath.Separator)) {
			return fmt.Errorf("归档中包含非法路径: %s", header.Name)
		}
		target := filepath.Join(dir, name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := writeExtractedFile(target, tr, os.FileMode(header.Mode)&0o777); err != nil {
				return err
			}
			count++
		case tar.TypeSymlink:
			linkTarget := filepath.FromSlash(header.Linkname)
			if filepath.IsAbs(linkTarget) {
				return fmt.Errorf("归档中包含非法符号链接: %s", header.Name)
			}
			resolved := filepath.Clean(filepath.Join(filepath.Dir(target), linkTarget))
			rel, err := filepath.Rel(dir, resolved)
			if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("归档中包含越界符号链接: %s", header.Name)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			if err := os.Symlink(linkTarget, target); err != nil {
				return err
			}
			count++
		}
	}
}

func writeExtractedFile(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Chmod(path, mode)
}

// swapUpdateDir atomically moves entries from stage into target, rolling back
// if any rename fails.
func swapUpdateDir(stage, target string) error {
	entries, err := os.ReadDir(stage)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return errors.New("更新归档为空")
	}
	suffix := ".nextai-old-" + strconv.Itoa(os.Getpid())
	var moved, placed [][2]string
	rollback := func() {
		for i := len(placed) - 1; i >= 0; i-- {
			_ = os.Rename(placed[i][1], placed[i][0])
		}
		for i := len(moved) - 1; i >= 0; i-- {
			_ = os.Rename(moved[i][1], moved[i][0])
		}
	}
	for _, entry := range entries {
		src := filepath.Join(stage, entry.Name())
		dst := filepath.Join(target, entry.Name())
		if _, err := os.Lstat(dst); err == nil {
			aside := filepath.Join(target, "."+entry.Name()+suffix)
			if err := os.Rename(dst, aside); err != nil {
				rollback()
				return err
			}
			moved = append(moved, [2]string{dst, aside})
		}
		if err := os.Rename(src, dst); err != nil {
			rollback()
			return err
		}
		placed = append(placed, [2]string{src, dst})
	}
	for _, m := range moved {
		_ = os.RemoveAll(m[1])
	}
	return nil
}
