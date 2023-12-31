package main

import (
	"errors"
	"strings"
)

type config struct {
	APIKey   string `envconfig:"API_KEY"`
	Token    string `envconfig:"VK_TOKEN"`
	GroupID  int    `envconfig:"GROUP_ID"`
	DataBase string `envconfig:"DATABASE"`
}

func (c *config) validate() error {
	if c.APIKey = strings.TrimSpace(c.APIKey); c.APIKey == "" {
		return errors.New("API_KEY required")
	}

	if c.Token = strings.TrimSpace(c.Token); c.Token == "" {
		return errors.New("VK_TOKEN required")
	}

	if c.GroupID == 0 {
		return errors.New("GROUP_ID required")
	} else if c.GroupID < 0 {
		return errors.New("GROUP_ID must be positive")
	}

	if c.DataBase = strings.TrimSpace(c.DataBase); c.DataBase == "" {
		return errors.New("DATABASE required")
	}

	return nil
}
