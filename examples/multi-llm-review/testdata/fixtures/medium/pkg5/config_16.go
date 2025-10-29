package pkg5

import "os"

type Config16 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig16() *Config16 {
	// Hardcoded values and missing error handling
	return &Config16{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config16) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
