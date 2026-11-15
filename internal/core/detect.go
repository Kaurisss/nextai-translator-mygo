package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

func detectLanguage(ctx context.Context, client *http.Client, engine, text string) string {
	characters := []rune(text)
	if len(characters) > 1000 {
		text = string(characters[:1000])
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	code, err := detectRemoteLanguage(ctx, client, engine, text)
	if err == nil {
		switch strings.ToLower(code) {
		case "zh", "zh-cn", "zh-hans":
			code = "zh-Hans"
		case "cht", "zh-tw", "zh-hant":
			code = "zh-Hant"
		case "jp":
			code = "ja"
		case "kor":
			code = "ko"
		case "fra":
			code = "fr"
		case "spa":
			code = "es"
		case "vie":
			code = "vi"
		case "per":
			code = "fa"
		}
		if language, ok := promptLanguages[code]; ok {
			return language.Code
		}
	}
	return detectPromptLanguage(text)
}

func detectRemoteLanguage(ctx context.Context, client *http.Client, engine, text string) (string, error) {
	var method, address string
	var body io.Reader
	var token string
	switch engine {
	case "百度":
		method = http.MethodPost
		address = "https://fanyi.baidu.com/langdetect?" + url.Values{"query": {text}}.Encode()
	case "谷歌":
		method = http.MethodGet
		query := url.Values{"client": {"gtx"}, "sl": {"auto"}, "tl": {"zh-CN"}, "dt": {"t"}, "q": {text}}
		address = "https://translate.google.com/translate_a/single?" + query.Encode()
	case "必应":
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://edge.microsoft.com/translate/auth", nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 Edg/113.0.1774.42")
		data, err := detectionResponse(client, req)
		if err != nil {
			return "", err
		}
		token = string(data)
		method = http.MethodPost
		address = "https://api-edge.cognitive.microsofttranslator.com/detect?api-version=3.0"
		data, err = json.Marshal([]map[string]string{{"Text": text}})
		if err != nil {
			return "", err
		}
		body = bytes.NewReader(data)
	default:
		return "", fmt.Errorf("不支持的语种检测引擎: %s", engine)
	}
	req, err := http.NewRequestWithContext(ctx, method, address, body)
	if err != nil {
		return "", err
	}
	if engine == "必应" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("User-Agent", "Mozilla/5.0 Edg/113.0.1774.42")
	}
	data, err := detectionResponse(client, req)
	if err != nil {
		return "", err
	}
	switch engine {
	case "百度":
		var response struct {
			Language string `json:"lan"`
		}
		err = json.Unmarshal(data, &response)
		return response.Language, err
	case "谷歌":
		var response []json.RawMessage
		if err := json.Unmarshal(data, &response); err != nil {
			return "", err
		}
		if len(response) < 3 {
			return "", fmt.Errorf("语种检测响应缺少语言")
		}
		var language string
		err = json.Unmarshal(response[2], &language)
		return language, err
	default:
		var response []struct {
			Language string `json:"language"`
		}
		if err := json.Unmarshal(data, &response); err != nil {
			return "", err
		}
		if len(response) == 0 {
			return "", fmt.Errorf("语种检测响应缺少语言")
		}
		return response[0].Language, nil
	}
}

func detectionResponse(client *http.Client, req *http.Request) ([]byte, error) {
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("语种检测 HTTP %d", response.StatusCode)
	}
	return io.ReadAll(io.LimitReader(response.Body, 1024*1024))
}
