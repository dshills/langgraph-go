package pkg5

import "os"

type Config19 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig19() *Config19 {
	// Hardcoded values and missing error handling
	return &Config19{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config19) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
