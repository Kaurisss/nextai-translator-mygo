package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/netproxy"
	"golang.org/x/net/http/httpproxy"
)

func HTTPClient(config ProxySettings, password string) (*http.Client, error) {
	transport := netproxy.Transport()
	if config.Enabled {
		port, err := strconv.Atoi(config.Port)
		if err != nil || port < 1 || port > 65535 || strings.TrimSpace(config.Server) == "" {
			return nil, errors.New("代理服务器和端口无效")
		}
		scheme := strings.ToLower(config.Protocol)
		if scheme != "http" && scheme != "https" && scheme != "socks5" {
			return nil, errors.New("代理协议须为 HTTP、HTTPS 或 SOCKS5")
		}
		u := url.URL{Scheme: scheme, Host: net.JoinHostPort(config.Server, config.Port)}
		if config.Username != "" {
			u.User = url.UserPassword(config.Username, password)
		}
		proxyConfig := httpproxy.Config{HTTPProxy: u.String(), HTTPSProxy: u.String(), NoProxy: config.NoProxy}
		proxyFunc := proxyConfig.ProxyFunc()
		transport = http.DefaultTransport.(*http.Transport).Clone()
		transport.Proxy = func(r *http.Request) (*url.URL, error) { return proxyFunc(r.URL) }
	}
	return &http.Client{Transport: transport, Timeout: 5 * time.Minute}, nil
}

func APIKey(keys string) string {
	var candidates []string
	for _, key := range strings.Split(keys, ",") {
		if key = strings.TrimSpace(key); key != "" {
			candidates = append(candidates, key)
		}
	}
	if len(candidates) == 0 {
		return ""
	}
	return candidates[rand.IntN(len(candidates))]
}

func SplitEndpoint(endpoint string) (string, string) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Host == "" {
		return endpoint, ""
	}
	base := u.Scheme + "://" + u.Host
	return base, strings.TrimPrefix(endpoint, base)
}

func JoinEndpoint(base, path string) string {
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")
}

type ModelList struct {
	Names    []string
	Thinking map[string]bool
}

func ListModels(ctx context.Context, client *http.Client, p Profile, key string) (*ModelList, error) {
	provider, ok := FindProvider(p.Provider)
	if !ok {
		return nil, errors.New("未知模型服务")
	}
	u, err := url.Parse(p.Endpoint)
	if err != nil || u.Host == "" {
		return nil, errors.New("API 地址无效")
	}
	switch provider.Protocol {
	case "gemini":
		u.Path = strings.TrimRight(u.Path, "/") + "/models"
	case "claude":
		u.Path = strings.TrimSuffix(u.Path, "/messages") + "/models"
	case "azure":
		return &ModelList{Names: []string{p.Model}}, nil
	case "cohere":
		u.Path = "/v1/models"
	default:
		u.Path = strings.TrimSuffix(u.Path, "/chat/completions") + "/models"
	}
	u.RawQuery = ""
	if provider.Protocol == "gemini" {
		query := u.Query()
		query.Set("pageSize", "1000")
		u.RawQuery = query.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	setAuth(req, provider.Protocol, APIKey(key))
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("获取模型列表: HTTP %d", response.StatusCode)
	}
	var body struct {
		Data   []struct{ ID string }
		Models []struct {
			ID, Name                   string
			Thinking                   bool
			SupportedGenerationMethods []string
		}
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4<<20)).Decode(&body); err != nil {
		return nil, err
	}
	var names []string
	var thinking map[string]bool
	if provider.Protocol == "gemini" {
		thinking = map[string]bool{}
	}
	for _, m := range body.Data {
		if m.ID != "" {
			names = append(names, m.ID)
		}
	}
	for _, m := range body.Models {
		name := m.ID
		if name == "" {
			name = strings.TrimPrefix(m.Name, "models/")
		}
		if provider.Protocol == "gemini" {
			thinking[name] = m.Thinking
			if m.SupportedGenerationMethods != nil && !slices.Contains(m.SupportedGenerationMethods, "generateContent") {
				continue
			}
		}
		if name != "" {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		return nil, errors.New("服务未返回模型；可启用无模型 API 支持并手动输入模型")
	}
	slices.Sort(names)
	return &ModelList{Names: slices.Compact(names), Thinking: thinking}, nil
}

func setAuth(req *http.Request, protocol, key string) {
	switch protocol {
	case "claude":
		req.Header.Set("x-api-key", key)
		req.Header.Set("anthropic-version", "2023-06-01")
	case "gemini":
		req.Header.Set("x-goog-api-key", key)
	case "azure":
		req.Header.Set("api-key", key)
	default:
		if key != "" {
			req.Header.Set("Authorization", "Bearer "+key)
		}
	}
}
