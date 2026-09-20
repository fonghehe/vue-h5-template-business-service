package config

import (
	"errors"
	"fmt"
	"os"

	"github.com/joho/godotenv"
)

// LoadDotEnv reads an optional project configuration file. Existing process
// variables take precedence, including explicitly empty values. Call this at
// process startup, before Load; configuration tests can keep using Load alone.
func LoadDotEnv(path string) error {
	values, err := godotenv.Read(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		// Parser errors may contain values from the file, including secrets.
		return fmt.Errorf("load %s: file is unreadable or has invalid dotenv syntax", path)
	}
	for key, value := range values {
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("load %s: invalid environment variable name", path)
		}
	}
	return nil
}
