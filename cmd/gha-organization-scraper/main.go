package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	githubanalysis "github.com/benfred/github-analysis"
	"github.com/benfred/github-analysis/config"

	"github.com/google/go-github/github"

	"golang.org/x/oauth2"
)

type fetchRequest struct {
	id   int64
	name string
}

type fetchResponse struct {
	statusCode int
	id         int64
	name       string
	response   *github.Response
	users      []*github.User
}

func writeOrganizationMembers(ctx context.Context, wg *sync.WaitGroup, responses chan fetchResponse, db *githubanalysis.Database) {
	defer wg.Done()

Loop:
	for {
		select {
		case <-ctx.Done():
			fmt.Printf("Not ok!\n")
			break Loop
		case response, ok := <-responses:
			if !ok {
				break Loop
			}
			err := db.InsertOrganizationMembers(response.id, response.name, response.users, response.statusCode, time.Now(), true)
			if err != nil {
				panic(err)
			}
		}
	}
	fmt.Printf("Exiting writer worker\n")
}

// createGithubClient with access tokens defined in cred
func createGithubClient(ctx context.Context, cred config.GitHubCredentials) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cred.Token})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

// request fetchRequest, output chan fetchResponse)
func fetchOrganization(ctx context.Context, client *github.Client,
	request fetchRequest, output chan fetchResponse) (*github.Response, error) {

	options := github.ListMembersOptions{PublicOnly: true}
	var responses fetchResponse
	firstPage := true

	for {
		users, resp, err := client.Organizations.ListMembers(ctx, request.name, &options)
		if err != nil && resp == nil {
			return nil, err
		}

		if firstPage {
			firstPage = false
			responses = fetchResponse{resp.StatusCode, request.id, request.name, resp, users}
		} else {
			// just append the rest of the users back here
			responses.users = append(responses.users, users...)
		}

		if resp.NextPage == 0 {
			output <- responses
			return resp, nil
		}
		options.Page = resp.NextPage
	}
}

func fetchOrganizations(ctx context.Context, wg *sync.WaitGroup, cred config.GitHubCredentials,
	requests chan fetchRequest, output chan fetchResponse) {
	defer wg.Done()
	client := createGithubClient(ctx, cred)
Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		case request, ok := <-requests:
			if !ok {
				break Loop
			}

			fmt.Printf("Fetching '%s'\n", request.name)
			resp, err := fetchOrganization(ctx, client, request, output)
			if err != nil {
				// If we got an error, and the context has been cancelled don't sweat it
				select {
				case <-ctx.Done():
					break Loop
				default:
				}

				// Just keep on trying rather than exitting and potentially dying
				fmt.Printf("Error fetching '%s': %s\n", request.name, err.Error())
				continue
			}

			if resp.Rate.Remaining < 250 {
				// Sleep until a couple seconds after the reset time
				sleepTime := resp.Rate.Reset.Sub(time.Now()) + time.Second*10
				fmt.Printf("Sleeping for %ds (Remaining requests=%d)\n", sleepTime/time.Second, resp.Rate.Remaining)
				select {
				case <-ctx.Done():
					break Loop
				case <-time.After(sleepTime):
				}
			}
		}
	}
	fmt.Printf("Exitting fetch worker: %s\n", cred.Account)
}

func queueOrganizations(ctx context.Context, db *githubanalysis.Database, requests chan fetchRequest) error {
	// Get a list of organizations to fetch from the db
	sql := `select users.id, users.login from users inner join repos on repos.ownerid = users.id
	left join organization_members on organization = users.id
	where type = 'Organization' and organization is null group by users.id order by sum(stars) desc`

	rows, err := db.Query(sql)
	if err != nil {
		return err
	}

	defer rows.Close()
	for rows.Next() {
		var userid int64
		var login string
		if err := rows.Scan(&userid, &login); err != nil {
			return err
		}

		fmt.Printf("userid %d login %s\n", userid, login)
		requests <- fetchRequest{userid, login}
	}

	return nil
}

func main() {
	// try to cleanup gracefully on system interrupts
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
	}()

	cfg := config.Read("config.toml")
	db, err := githubanalysis.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}

	requests := make(chan fetchRequest, 100)
	output := make(chan fetchResponse)

	err = queueOrganizations(ctx, db, requests)
	if err != nil {
		log.Fatal(err)
	}

	// create a goroutine per credential to handle making api requests
	var wg sync.WaitGroup
	for _, cred := range cfg.GitHubCredentials {
		wg.Add(1)
		go fetchOrganizations(ctx, &wg, cred, requests, output)
	}

	// create a single goroutine for writing results to db/disk
	var outputWG sync.WaitGroup
	outputWG.Add(1)
	go writeOrganizationMembers(ctx, &outputWG, output, db)

	close(requests)
	wg.Wait()
	close(output)
	outputWG.Wait()
}
