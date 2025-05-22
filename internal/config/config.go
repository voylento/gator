package config

import (
	"encoding/json"
	"fmt"
	"os"
)
const (
	configFileName = ".gatorconfig.json"
)

type Config struct{
	DbUrl				string	`json:"db_url"`
	UserName		string	`json:"user_name"`
}


func LoadConfig() (*Config, error) {
	file, err := os.Open(getConfigFilePath())
	if err != nil {
		return nil, fmt.Errorf("Error reading config file: %v\n", err)
	}
	defer file.Close()

	var config Config
	
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("Error decoding config file: %v", err)
	}

	return &config, nil
}

func (c *Config) SetUser(user string) {
	c.UserName = user
	file, err := os.Create(getConfigFilePath())
	if err != nil {
		fmt.Printf("Error opening config file: %v\n", err)
		panic(err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")

	err = encoder.Encode(c)
	if err != nil {
		fmt.Printf("Error encoding config: %v", err)
		panic(err)
	}
}

func getConfigFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting user home directory: %v", err)
		panic(err)
	}

	return homeDir + "/" + configFileName
}
