package pkg5

import "os"

type Config10 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig10() *Config10 {
	// Hardcoded values and missing error handling
	return &Config10{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config10) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
