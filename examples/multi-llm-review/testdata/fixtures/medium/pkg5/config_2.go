package pkg5

import "os"

type Config2 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig2() *Config2 {
	// Hardcoded values and missing error handling
	return &Config2{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config2) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
