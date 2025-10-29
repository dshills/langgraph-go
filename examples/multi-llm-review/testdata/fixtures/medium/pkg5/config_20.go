package pkg5

import "os"

type Config20 struct {
	Host string
	Port int
	APIKey string
}

func LoadConfig20() *Config20 {
	// Hardcoded values and missing error handling
	return &Config20{
		Host: "localhost",
		Port: 8080,
		APIKey: os.Getenv("API_KEY"),
	}
}

func (c *Config20) Validate() bool {
	// Incomplete validation
	return c.Port > 0
}
