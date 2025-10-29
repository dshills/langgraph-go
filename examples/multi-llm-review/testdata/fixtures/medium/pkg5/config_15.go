package pkg5

import "os"

type Config15 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig15() *Config15 {
	// Hardcoded values and missing error handling
	return &Config15{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config15) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
