package pkg5

import "os"

type Config5 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig5() *Config5 {
	// Hardcoded values and missing error handling
	return &Config5{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config5) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
