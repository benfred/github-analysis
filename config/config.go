package config

import (
	"log"

	"github.com/BurntSushi/toml"
)

// Config for this package
type Config struct {
	GithubarchivePath string
	Database          Database
	GitHubCredentials []GitHubCredentials
	GoogleMapsKey     string
}

// Database defines the login credentials for the metadata in the db
type Database struct {
	Host     string
	Username string
	Password string
	DBName   string
	Port     int
}

// GitHubCredentials defines a single api token for accessing the github api
type GitHubCredentials struct {
	Account string
	Token   string
}

// Read config from a TOML file
func Read(filename string) Config {
	var ret Config
	_, err := toml.DecodeFile(filename, &ret)
	if err != nil {
		log.Fatal(err)
	}
	return ret
}
