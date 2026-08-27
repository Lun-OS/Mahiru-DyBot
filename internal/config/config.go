package config

import (
	"encoding/json"
	"fmt"
	"os"
)

type Config struct {
	ListenAddr     string `json:"listen_addr"`
	UdpAddr        string `json:"udp_addr"` // 桌面客户端 UDP 画面/控制端口
	AccessToken    string `json:"access_token"` // 已废弃：OneBot 授权改由 WebUI 运行时设置管理
	Headless       bool   `json:"headless"`
	CdpURL         string `json:"cdp_url"`
	BrowserChannel string `json:"browser_channel"`
	StorageDir     string `json:"storage_dir"`
	WsPath         string `json:"ws_path"`
	UserAgent      string `json:"user_agent"` // 自定义UA；留空则各账号独立随机生成并固定
}

func Load(path string) (*Config, error) {
	cfg := &Config{
		ListenAddr:     ":17836",
		UdpAddr:        ":17837",
		Headless:       true,
		BrowserChannel: "chrome",
		StorageDir:     "./storage",
		WsPath:         "/event",
	}
	data, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("解析配置文件失败: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	return cfg, nil
}

func Save(cfg *Config, path string) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}
