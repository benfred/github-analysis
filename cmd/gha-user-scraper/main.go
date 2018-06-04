package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	githubanalysis "github.com/benfred/github-analysis"
	"github.com/benfred/github-analysis/config"

	"github.com/google/go-github/github"

	"golang.org/x/oauth2"
)

// TODO: this is mostly cut-n-paste from gha-scraper. refactor
// json write is an obvious one, aside from that its a little tricky

type fetchRequest struct {
	id   int64
	name string
}

type fetchResponse struct {
	statusCode int
	id         int64
	name       string
	response   *github.Response
	user       *github.User
}

func writeUsers(ctx context.Context, wg *sync.WaitGroup, responses chan fetchResponse, db *githubanalysis.Database, jsonOutputPath string) {
	defer wg.Done()

	var f *os.File
	var gz *gzip.Writer
	written := 0

Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		case response, ok := <-responses:
			if !ok {
				break Loop
			}
			fmt.Printf("Writing: %s\n", response.name)
			time := time.Now()

			if response.user == nil {
				// If we dont' have a github.User object, probably failed to fetch
				// update DB with the statuscode in that case
				db.InsertUserStatus(response.id, response.name, response.statusCode, time)
			} else {
				if response.id != int64(response.user.GetID()) && response.id != -123 {
					// If github returned a different userid than the one we expected for this
					// name, that means that the userid has been deleted/replaced with a different
					// one of the same name. Update DB so we don't try scraping again
					// For users this should be exceptionally rare
					fmt.Printf("id mistmatch on %s\n", response.name)
					db.InsertUserStatus(response.id, response.name, 404, time)
				}

				db.InsertUser(&response.statusCode, &time, response.user, true)
			}

			if response.user != nil && response.statusCode == 200 {
				// TODO: move this into a class, create a 'ndjson' package ...
				if f == nil {
					now := time.Unix()
					filename := path.Join(jsonOutputPath, fmt.Sprintf("github_users_%d.json.gz", now))
					fmt.Printf("Writing json to %s\n", filename)

					var err error
					f, err = os.Create(filename)
					if err != nil {
						panic(err)
					}
					gz = gzip.NewWriter(f)
				}

				bytes, err := json.Marshal(response.user)
				if err != nil {
					panic(err) // TODO: is this valid?
				}
				gz.Write(bytes)
				gz.Write([]byte("\n"))

				// Reset the file if it gets too large
				written += len(bytes) + 1
				if written >= 100000000 {
					gz.Close()
					f.Close()
					gz = nil
					f = nil
					written = 0
				}
			}
		}
	}
	if gz != nil {
		gz.Close()
	}
	if f != nil {
		f.Close()
	}
	fmt.Printf("Exiting writer worker")
}

// createGithubClient with access tokens defined in cred
func createGithubClient(ctx context.Context, cred config.GitHubCredentials) *github.Client {
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: cred.Token})
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}

func fetchUser(ctx context.Context, client *github.Client, request fetchRequest, output chan fetchResponse) (*github.Response, error) {
	// Timeout this request after 20 seconds
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	user, resp, err := client.Users.Get(ctx, request.name)
	if err != nil && resp == nil {
		// only return an error if we don't have a response, otherwise
		// we want to insert that statuscode into the db.
		return nil, err
	}

	output <- fetchResponse{resp.StatusCode, request.id, request.name, resp, user}
	return resp, nil
}

func fetchUsers(ctx context.Context, wg *sync.WaitGroup, cred config.GitHubCredentials,
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
			resp, err := fetchUser(ctx, client, request, output)
			if err != nil {
				// If we got an error, and the context has been cancelled don't sweat it
				select {
				case <-ctx.Done():
					break Loop
				default:
				}

				// TODO: better error handling
				fmt.Printf("Error fetching '%s': %s\n", request.name, err.Error())

				// Just keep on trying rather than exitting and potentially dying
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

func queueUsers(ctx context.Context, db *githubanalysis.Database,
	filename string, requests chan fetchRequest, refetch bool) error {
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	var scanner *bufio.Scanner
	if strings.HasSuffix(filename, ".gz") {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gr.Close()
		scanner = bufio.NewScanner(gr)
	} else {
		scanner = bufio.NewScanner(f)
	}

	for scanner.Scan() {
		tokens := strings.Fields(scanner.Text())
		if len(tokens) == 3 {
			userid, err := strconv.ParseInt(tokens[1], 10, 64)
			if err != nil {
				return err
			}
			if !refetch {
				hasuser, err := db.HasUser(userid)
				if err != nil {
					fmt.Printf("Failed to query user status '%s': %s", tokens[2], err.Error())
				}

				if hasuser {
					// fmt.Printf("Skipping %s\n", tokens[2])
					continue
				}
			}
			select {
			case <-ctx.Done():
				return nil
			case requests <- fetchRequest{userid, tokens[2]}:
			}
		}

	}
	return nil
}

func main() {
	jsonpath := flag.String("jsonpath", "", "location of json files")
	filename := flag.String("filename", "", "Filename to process")
	refetch := flag.Bool("refetch", false, "Refetch users that have already been stored in the database")
	flag.Parse()
	if *filename == "" && *jsonpath == "" {
		flag.Usage()
		os.Exit(1)
	}

	cfg := config.Read("config.toml")

	db, err := githubanalysis.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}

	// try to cleanup gracefully on system interrupts
	ctx, cancel := context.WithCancel(context.Background())
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		cancel()
	}()

	requests := make(chan fetchRequest, 100)
	output := make(chan fetchResponse)

	// create a goroutine per credential to handle making api requests
	var wg sync.WaitGroup
	for _, cred := range cfg.GitHubCredentials {
		wg.Add(1)
		go fetchUsers(ctx, &wg, cred, requests, output)
	}

	// create a single goroutine for writing results to db/disk
	var outputWG sync.WaitGroup
	outputWG.Add(1)
	go writeUsers(ctx, &outputWG, output, db, *jsonpath)

	err = queueUsers(ctx, db, *filename, requests, *refetch)
	if err != nil {
		panic(err)
	}

	close(requests)
	wg.Wait()
	close(output)
	outputWG.Wait()
}
