package desktop

import (
	"context"
	"os"
	"strings"

	"github.com/egoist/mygo/audio"
	"github.com/Kaurisss/nextai-translator-mygo/internal/core"
)

type Voice struct{ Name, Language string }

func SpeechVoice(options core.SpeechSettings, language string) string {
	for _, v := range options.Voices {
		provider := v.Provider
		if provider == "" {
			provider = "系统默认"
		}
		if provider != options.Provider {
			continue
		}
		if v.Language == language || core.LanguageCode(v.Language, "") == language {
			if v.Name == "系统默认" {
				return ""
			}
			return v.Name
		}
	}
	return ""
}

func PlaySpeech(ctx context.Context, path string, volume float64) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	player, err := audio.OpenStream(f)
	if err != nil {
		f.Close()
		return err
	}
	defer player.Close()
	done := make(chan struct{})
	player.OnEnd(func() { close(done) })
	player.SetVolume(float64(volume / 100)).Play()
	select {
	case <-ctx.Done():
		player.Stop()
		return ctx.Err()
	case <-done:
		return nil
	}
}

func VoiceLanguages(voices []Voice, code string) []string {
	result := []string{"系统默认"}
	for _, voice := range voices {
		if strings.Split(voice.Language, "-")[0] == strings.Split(code, "-")[0] {
			result = append(result, voice.Name)
		}
	}
	return result
}
