package main

import (
	"encoding/json"
	"os"
)

// ===================== 配置管理 =====================

func loadConfig() {
	configLock.Lock()
	defer configLock.Unlock()

	// 默认配置
	config = Config{
		DefaultInterval: 60,
		MaxRetries:      3,
	}

	data, err := os.ReadFile(ConfigFile)
	if err == nil {
		json.Unmarshal(data, &config)
	} else {
		saveConfigNoLock()
	}
}

func saveConfigNoLock() {
	data, _ := json.MarshalIndent(config, "", "  ")
	os.WriteFile(ConfigFile, data, 0644)
}

func saveConfig() {
	configLock.Lock()
	defer configLock.Unlock()
	saveConfigNoLock()
}
