package pkg5

import "os"

type Config12 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig12() *Config12 {
	// Hardcoded values and missing error handling
	return &Config12{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config12) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
