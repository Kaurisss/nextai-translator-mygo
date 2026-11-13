package desktop

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
)

func TestSpeechResourceChecksum(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, "model-data") }))
	defer server.Close()
	hash := sha256.Sum256([]byte("model-data"))
	path := filepath.Join(t.TempDir(), "model")
	if err := downloadSpeechAsset(context.Background(), server.Client(), server.URL, path, hex.EncodeToString(hash[:]), func(string) {}); err != nil {
		t.Fatal(err)
	}
	if err := downloadSpeechAsset(context.Background(), server.Client(), server.URL, path, strings.Repeat("0", 64), func(string) {}); err == nil {
		t.Fatal("wrong model checksum accepted")
	}
}

func TestSpeechArchiveRejectsEscape(t *testing.T) {
	for _, name := range []string{"model/model.onnx", "../outside", "/absolute", `model\..\outside`} {
		var b bytes.Buffer
		writer := tar.NewWriter(&b)
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: 0600, Size: 4}); err != nil {
			t.Fatal(err)
		}
		writer.Write([]byte("data"))
		writer.Close()
		root := t.TempDir()
		err := extractSpeechArchive(context.Background(), &b, root)
		if name == "model/model.onnx" {
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(root, name))
			if err != nil || string(data) != "data" {
				t.Fatalf("extracted %q %v", data, err)
			}
		} else if err == nil {
			t.Errorf("accepted archive path %s", name)
		}
	}
}

func TestLocalSpeechModelsAndArguments(t *testing.T) {
	for _, tc := range []struct{ text, lang, want string }{{"Hello", "en", "en"}, {"Hello", "zh-Hans", "zh"}, {"你好", "en", "zh"}, {"こんにちは", "ja", ""}} {
		got, err := localSpeechModel(tc.text, tc.lang)
		if got != tc.want || ((tc.want == "") != (err != nil)) {
			t.Errorf("%+v got %s %v", tc, got, err)
		}
	}
	args := localSpeechArgs(t.TempDir(), "en", `Hello "world"; $(x)`, "test.wav", 1.25)
	if args[len(args)-1] != `Hello "world"; $(x)` || !strings.Contains(strings.Join(args, " "), "--kokoro-model=") {
		t.Fatal(args)
	}
	if SpeechVoice(core.SpeechSettings{Voices: []core.SpeechVoice{{Language: "English", Name: "系统默认"}}}, "English") != "" {
		t.Fatal("system default passed as literal voice name")
	}
}

func TestWritingNewlineChord(t *testing.T) {
	mods, key, err := ParseChord("Ctrl+Shift+Enter")
	if err != nil || key != 13 || len(mods) != 2 {
		t.Fatalf("%v %v %v", mods, key, err)
	}
	if _, _, err := ParseChord("Shift+Unknown"); err == nil {
		t.Fatal("invalid newline chord accepted")
	}
}

func TestLocalSpeechIntegration(t *testing.T) {
	root := os.Getenv("NEXTAI_LOCAL_TTS_TEST_ROOT")
	if root == "" {
		t.Skip("set NEXTAI_LOCAL_TTS_TEST_ROOT to verified runtime and model directories")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	options := core.DefaultData().Settings.Speech
	options.Volume = 0
	options.Rate = 1.25
	for _, sample := range []struct{ text, language string }{{"This is a native speech synthesis test.", "en"}, {"你好，这是原生语音合成测试。", "zh-Hans"}} {
		t.Run(sample.language, func(t *testing.T) {
			if err := LocalSpeak(ctx, http.DefaultClient, root, sample.text, sample.language, options, "", func(message string) { t.Log(message) }); err != nil {
				t.Fatal(err)
			}
		})
	}
}
