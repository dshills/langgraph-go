package pkg5

import "os"

type Config13 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig13() *Config13 {
	// Hardcoded values and missing error handling
	return &Config13{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config13) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
