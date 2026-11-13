package desktop

import (
	"archive/tar"
	"compress/bzip2"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
)

type speechAsset struct {
	directory, hash string
	urls            []string
}

var speechModels = map[string]speechAsset{
	"zh": {directory: "vits-melo-tts-zh_en", hash: "e58351ed7149f290a54534538badd4077cdbe6fddc964b24d0bee870415d1514", urls: []string{"https://api.github.com/repos/k2-fsa/sherpa-onnx/releases/assets/203769960"}},
	"en": {directory: "kokoro-en-v0_19", hash: "912804855a04745fa77a30be545b3f9a5d15c4d66db00b88cbcd4921df605ac7", urls: []string{"https://api.github.com/repos/k2-fsa/sherpa-onnx/releases/assets/281054203"}},
}

func localSpeechModel(text, language string) (string, error) {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return "zh", nil
		}
	}
	if strings.HasPrefix(language, "en") {
		return "en", nil
	}
	switch language {
	case "zh-Hans", "zh-Hant", "yue", "lzh":
		return "zh", nil
	}
	return "", fmt.Errorf("本地 TTS 不支持 %s，请使用系统默认语音", language)
}

func speechRuntime() (speechAsset, error) {
	var directory, hash, id string
	switch runtime.GOOS + "/" + runtime.GOARCH {
	case "windows/amd64":
		directory = "sherpa-onnx-v1.13.4-win-x64-shared-MT-Release"
		hash = "c312b30e55258d291067537ce4a6f90e155f1c16b6a10381a5729924e98b7879"
		id = "469140988"
	case "linux/amd64":
		directory = "sherpa-onnx-v1.13.4-linux-x64-shared"
		hash = "18887dc13c7d313d0e0f6c164ed31715c27c1c2c4f71acd7c0147dc84cf02514"
		id = "469113304"
	case "darwin/amd64", "darwin/arm64":
		directory = "sherpa-onnx-v1.13.4-osx-universal2-shared"
		hash = "02b9b0cf30819a18c6d5cf861aebf32336cb79958ab97a2b248227059678058b"
		id = "469143793"
	default:
		return speechAsset{}, fmt.Errorf("本地 TTS 运行时暂不支持 %s/%s", runtime.GOOS, runtime.GOARCH)
	}
	return speechAsset{directory: directory, hash: hash, urls: []string{"https://api.github.com/repos/k2-fsa/sherpa-onnx/releases/assets/" + id, "https://github.com/k2-fsa/sherpa-onnx/releases/download/v1.13.4/" + directory + ".tar.bz2"}}, nil
}

func ensureSpeechAsset(ctx context.Context, client *http.Client, root string, asset speechAsset, report func(string)) error {
	if _, err := os.Stat(filepath.Join(root, asset.directory, ".verified")); err == nil {
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(root, ".download-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	var downloadErr error
	for _, address := range asset.urls {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		report("正在下载 " + asset.directory)
		downloadErr = downloadSpeechAsset(ctx, client, address, filepath.Join(temp, "archive"), asset.hash, report)
		if downloadErr == nil {
			break
		}
	}
	if downloadErr != nil {
		return downloadErr
	}
	report("正在解压 " + asset.directory)
	f, err := os.Open(filepath.Join(temp, "archive"))
	if err != nil {
		return err
	}
	err = extractSpeechArchive(ctx, bzip2.NewReader(f), temp)
	f.Close()
	if err != nil {
		return err
	}
	source := filepath.Join(temp, asset.directory)
	if err := os.WriteFile(filepath.Join(source, ".verified"), []byte(asset.hash), 0600); err != nil {
		return err
	}
	// A partially installed directory was never verified; keep it until the new download succeeds.
	destination := filepath.Join(root, asset.directory)
	if err := os.RemoveAll(destination); err != nil {
		return err
	}
	return os.Rename(source, destination)
}

func downloadSpeechAsset(ctx context.Context, client *http.Client, address, path, hash string, report func(string)) error {
	req, err := http.NewRequestWithContext(ctx, "GET", address, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/octet-stream")
	response, err := client.Do(req)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return fmt.Errorf("下载语音资源: HTTP %d", response.StatusCode)
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	hasher := sha256.New()
	var total int64
	last := time.Now()
	buffer := make([]byte, 128*1024)
	for {
		n, readErr := response.Body.Read(buffer)
		if n > 0 {
			if _, err := f.Write(buffer[:n]); err != nil {
				f.Close()
				return err
			}
			hasher.Write(buffer[:n])
			total += int64(n)
		}
		if time.Since(last) > time.Second {
			report(fmt.Sprintf("正在下载语音资源：%.1f MB", float64(total)/(1<<20)))
			last = time.Now()
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			f.Close()
			return readErr
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if hex.EncodeToString(hasher.Sum(nil)) != hash {
		return errors.New("语音资源 SHA-256 校验失败")
	}
	return nil
}

func extractSpeechArchive(ctx context.Context, r io.Reader, destination string) error {
	archive := tar.NewReader(r)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		header, err := archive.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		if !filepath.IsLocal(header.Name) || strings.Contains(header.Name, `\`) {
			return errors.New("语音资源包含无效路径")
		}
		path := filepath.Join(destination, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0700); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
				return err
			}
			f, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode)&0777)
			if err != nil {
				return err
			}
			_, err = io.Copy(f, archive)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		case tar.TypeSymlink, tar.TypeLink:
			if !filepath.IsLocal(filepath.Join(filepath.Dir(header.Name), header.Linkname)) {
				return errors.New("语音资源包含无效链接")
			}
			// Shared libraries can use relative symlinks; Windows archives contain regular DLLs.
			if header.Typeflag == tar.TypeSymlink {
				if err := os.Symlink(header.Linkname, path); err != nil {
					return err
				}
			} else {
				if err := os.Link(filepath.Join(destination, header.Linkname), path); err != nil {
					return err
				}
			}
		}
	}
}

func localSpeechArgs(root, model, text, output string, rate float64) []string {
	directory := filepath.Join(root, speechModels[model].directory)
	args := []string{"--num-threads=2", "--speed=" + strconv.FormatFloat(float64(rate), 'f', 2, 32), "--output-filename=" + output}
	if model == "zh" {
		args = append(args, "--vits-model="+filepath.Join(directory, "model.onnx"), "--vits-lexicon="+filepath.Join(directory, "lexicon.txt"), "--vits-tokens="+filepath.Join(directory, "tokens.txt"))
		var rules []string
		for _, name := range []string{"phone.fst", "date.fst", "number.fst"} {
			if _, err := os.Stat(filepath.Join(directory, name)); err == nil {
				rules = append(rules, filepath.Join(directory, name))
			}
		}
		if len(rules) > 0 {
			args = append(args, "--tts-rule-fsts="+strings.Join(rules, ","))
		}
	} else {
		args = append(args, "--kokoro-model="+filepath.Join(directory, "model.onnx"), "--kokoro-voices="+filepath.Join(directory, "voices.bin"), "--kokoro-tokens="+filepath.Join(directory, "tokens.txt"), "--kokoro-data-dir="+filepath.Join(directory, "espeak-ng-data"))
	}
	return append(args, "--", text)
}

func LocalSpeak(ctx context.Context, client *http.Client, root, text, language string, options core.SpeechSettings, voice string, report func(string)) error {
	model, err := localSpeechModel(text, language)
	if err != nil {
		return err
	}
	runtimeAsset, err := speechRuntime()
	if err != nil {
		return err
	}
	if err := ensureSpeechAsset(ctx, client, root, runtimeAsset, report); err != nil {
		return err
	}
	asset := speechModels[model]
	for _, host := range []string{"https://release.yansu.app/sherpa/tts/", "https://model-assets.yansu.app/sherpa/tts/", "https://github.com/k2-fsa/sherpa-onnx/releases/download/tts-models/"} {
		asset.urls = append(asset.urls, host+asset.directory+".tar.bz2")
	}
	if err := ensureSpeechAsset(ctx, client, root, asset, report); err != nil {
		return err
	}
	temp, err := os.MkdirTemp(root, "speech-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(temp)
	name := "sherpa-onnx-offline-tts"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := filepath.Join(root, runtimeAsset.directory, "bin", name)
	if _, err := os.Stat(binary); err != nil {
		return fmt.Errorf("本地 TTS 运行程序: %w", err)
	}
	runes := []rune(text)
	for len(runes) > 0 {
		count := min(len(runes), 500)
		if count < len(runes) {
			for n := count - 1; n > count/2; n-- {
				if unicode.IsSpace(runes[n]) || strings.ContainsRune("。！？.!?；;", runes[n]) {
					count = n + 1
					break
				}
			}
		}
		part := string(runes[:count])
		runes = runes[count:]
		output := filepath.Join(temp, "speech.wav")
		args := localSpeechArgs(root, model, part, output, options.Rate)
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.Env = append(os.Environ(), "PATH="+filepath.Join(root, runtimeAsset.directory, "lib")+string(os.PathListSeparator)+os.Getenv("PATH"), "LD_LIBRARY_PATH="+filepath.Join(root, runtimeAsset.directory, "lib"), "DYLD_LIBRARY_PATH="+filepath.Join(root, runtimeAsset.directory, "lib"))
		hideCommand(cmd)
		report("正在生成语音")
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("本地语音生成失败: %w: %s", err, strings.TrimSpace(string(out)))
		}
		if err := PlaySpeech(ctx, output, options.Volume); err != nil {
			return err
		}
	}
	return nil
}
