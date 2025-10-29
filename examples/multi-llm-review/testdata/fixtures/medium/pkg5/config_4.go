package pkg5

import "os"

type Config4 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig4() *Config4 {
	// Hardcoded values and missing error handling
	return &Config4{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config4) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
