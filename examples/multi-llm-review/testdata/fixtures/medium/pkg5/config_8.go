package pkg5

import "os"

type Config8 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig8() *Config8 {
	// Hardcoded values and missing error handling
	return &Config8{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config8) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
