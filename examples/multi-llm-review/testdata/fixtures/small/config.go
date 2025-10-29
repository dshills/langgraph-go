package main

import (
	"encoding/json"
	"os"
)

type Config struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Database string `json:"database"`
	APIKey   string `json:"api_key"`
}

var globalConfig *Config

// Config loading without error handling - intentional issue
func LoadConfig(filename string) {
	file, _ := os.Open(filename)
	
	globalConfig = &Config{}
	// Ignoring decode error
	json.NewDecoder(file).Decode(globalConfig)
	
	// Missing file.Close()
}

func GetConfig() *Config {
	if globalConfig == nil {
		LoadConfig("config.json")
	}
	return globalConfig
}
