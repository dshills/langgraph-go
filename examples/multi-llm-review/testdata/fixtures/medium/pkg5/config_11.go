package pkg5

import "os"

type Config11 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig11() *Config11 {
	// Hardcoded values and missing error handling
	return &Config11{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config11) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
