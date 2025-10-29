package pkg5

import "os"

type Config1 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig1() *Config1 {
	// Hardcoded values and missing error handling
	return &Config1{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config1) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
