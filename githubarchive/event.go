package githubarchive

import (
	"strings"

	"github.com/buger/jsonparser"
)

// Event holds parsed data about a single event from the Github Archive
type Event struct {
	Type         string
	RepoID       int64
	RepoName     string
	RepoLanguage string
	UserID       int64
	UserName     string
	ForkID       int64
	ForkName     string
	CreatedAt    string
}

func repoFromURL(url string) string {
	githubPrefix := "https://github.com/"
	if strings.HasPrefix(url, githubPrefix) {
		url = url[len(githubPrefix):]
	}
	return url
}

// ParseEvent parses the JSON and returns a new Event struct. The JSON
// can be from multiple different formats produced by github over the
// years
func ParseEvent(data []byte) *Event {
	// TODO int64 -> int?
	var repoID int64 = -1
	var userID int64 = -1

	// This is slightly complciated since we're trying to extract multiple
	// different JSON formats
	// TODO: jsonparser.EachKey api might be faster?
	eventType, _ := jsonparser.GetString(data, "type")

	created_at, _ := jsonparser.GetString(data, "created_at")

	repo, err := jsonparser.GetString(data, "repo", "name")
	if err != nil {
		repourl, err := jsonparser.GetString(data, "repository", "url")
		if err == nil {
			repo = repoFromURL(repourl)
		} else {
			repo, err = jsonparser.GetString(data, "repository", "full_name")
		}

		repoID, err = jsonparser.GetInt(data, "repository", "id")
		if err != nil {
			repoID = -1
		}
	} else {
		repoID, err = jsonparser.GetInt(data, "repo", "id")
		if err != nil {
			repoID = -1
		}
	}

	user, err := jsonparser.GetString(data, "actor", "login")
	if err != nil {
		user, err = jsonparser.GetString(data, "actor")
		if err != nil {
			user = "?"
		}
	} else {
		userID, err = jsonparser.GetInt(data, "actor", "id")
		if err != nil {
			userID = -1
		}
	}

	language, err := jsonparser.GetString(data, "repository", "language")
	if err != nil && (eventType == "PullRequestEvent") {
		language, _ = jsonparser.GetString(data, "payload", "pull_request", "base", "repo", "language")
	}

	return &Event{Type: eventType, RepoName: repo, RepoID: repoID, RepoLanguage: language,
		UserName: user, UserID: userID, CreatedAt: created_at}
}

// ParseForkEvent returns the forked repo name and forked repo id from a JSON githubarchive event
func ParseForkEvent(repo string, data []byte) (int64, string) {
	forkID, err := jsonparser.GetInt(data, "payload", "forkee", "id")
	if err != nil {
		forkID = -1
	}
	forkName, err := jsonparser.GetString(data, "payload", "forkee", "full_name")
	if err != nil {
		// 2013/2014: ForkEvent only stored in the url field, means no meaningful forkID =(
		url, err := jsonparser.GetString(data, "url")
		if err == nil {
			forkName = repoFromURL(url)
		} else {
			// 2012: forkID should already be set appropiately, forkName needs gotten from url
			url, err := jsonparser.GetString(data, "payload", "forkee", "html_url")
			if err == nil {
				forkName = repoFromURL(url)
			} else if repo != "/" {
				// 2011
				forkID, _ = jsonparser.GetInt(data, "payload", "forkee")
				// get the user that forked the repo, and swap into the repo name
				actor, err := jsonparser.GetString(data, "payload", "actor")
				if err == nil {
					tokens := strings.Split(repo, "/")
					tokens[0] = actor
					forkName = strings.Join(tokens, "/")
				}
			}
		}
	}

	return forkID, forkName
}
