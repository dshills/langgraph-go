package pkg5

import "os"

type Config14 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig14() *Config14 {
	// Hardcoded values and missing error handling
	return &Config14{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config14) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
