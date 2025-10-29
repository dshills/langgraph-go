package pkg5

import "os"

type Config18 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig18() *Config18 {
	// Hardcoded values and missing error handling
	return &Config18{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config18) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
