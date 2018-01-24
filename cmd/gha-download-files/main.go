package main

import (
	"github.com/benfred/github-analysis/config"
	"github.com/benfred/github-analysis/githubarchive"
)

func main() {
	cfg := config.Read("config.toml")
	githubarchive.DownloadFiles(cfg.GithubarchivePath)
}
