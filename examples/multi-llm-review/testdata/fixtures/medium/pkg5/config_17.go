package pkg5

import "os"

type Config17 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig17() *Config17 {
	// Hardcoded values and missing error handling
	return &Config17{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config17) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
