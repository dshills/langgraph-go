package pkg5

import "os"

type Config7 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig7() *Config7 {
	// Hardcoded values and missing error handling
	return &Config7{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config7) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
