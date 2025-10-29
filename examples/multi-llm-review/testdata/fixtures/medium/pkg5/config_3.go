package pkg5

import "os"

type Config3 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig3() *Config3 {
	// Hardcoded values and missing error handling
	return &Config3{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config3) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
