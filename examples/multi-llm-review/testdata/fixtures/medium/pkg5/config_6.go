package pkg5

import "os"

type Config6 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig6() *Config6 {
	// Hardcoded values and missing error handling
	return &Config6{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config6) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
