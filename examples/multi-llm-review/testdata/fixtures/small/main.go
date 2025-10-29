package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("Starting application...")
	
	// Missing error check - intentional issue
	file, _ := os.Open("config.json")
	
	data := make([]byte, 1024)
	file.Read(data)
	
	fmt.Printf("Read data: %s\n", string(data))
	
	// Missing defer file.Close()
	processConfig(string(data))
	
	fmt.Println("Application started successfully")
}

func processConfig(config string) {
	if config == "" {
		fmt.Println("Warning: empty config")
		return
	}
	fmt.Println("Config processed")
}
