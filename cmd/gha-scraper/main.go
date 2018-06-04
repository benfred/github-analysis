package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
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
	"github.com/benfred/github-analysis/githubarchive"

	"github.com/google/go-github/github"

	"golang.org/x/oauth2"
)

type fetchRequest struct {
	repoid   int64
	reponame string
}

type fetchedRepo struct {
	statusCode int
	repoid     int64
	reponame   string
	response   *github.Response
	repo       *github.Repository
}

func writeRepo(ctx context.Context, wg *sync.WaitGroup, repos chan fetchedRepo, db *githubanalysis.Database, jsonOutputPath string) {
	defer wg.Done()

	var f *os.File
	var gz *gzip.Writer
	written := 0

Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		case repo, ok := <-repos:
			if !ok {
				break Loop
			}
			fmt.Printf("Writing repo: %s\n", repo.reponame)
			time := time.Now()

			if repo.repo == nil {
				// If we dont' have a github.Repository object, probably failed to fetch
				// update DB with the statuscode in that case
				db.InsertRepoStatus(repo.repoid, repo.reponame, repo.statusCode, time)
			} else {
				if repo.repoid != int64(repo.repo.GetID()) {
					// If github returned a different repoid than the one we expected for this
					// name, that means that the repoid has been deleted/replaced with a different
					// one of the same name. Update DB so we don't try scraping again
					db.InsertRepoStatus(repo.repoid, repo.reponame, 404, time)
				}

				db.InsertRepo(&repo.statusCode, &time, repo.repo, true)
			}

			if repo.repo != nil && repo.statusCode == 200 {
				if f == nil {
					now := time.Unix()
					filename := path.Join(jsonOutputPath, fmt.Sprintf("github_repos_%d.json.gz", now))
					fmt.Printf("Writing json to %s\n", filename)

					var err error
					f, err = os.Create(filename)
					if err != nil {
						panic(err)
					}
					gz = gzip.NewWriter(f)
				}

				bytes, err := json.Marshal(repo.repo)
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

func fetchRepo(ctx context.Context, client *github.Client, request fetchRequest, output chan fetchedRepo) (*github.Response, error) {
	// Timeout this request after 20 seconds
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	tokens := strings.Split(request.reponame, "/")
	if len(tokens) != 2 {
		return nil, fmt.Errorf("Unknown repo type '%s'", request.reponame)
	}

	repo, resp, err := client.Repositories.Get(ctx, tokens[0], tokens[1])
	if err != nil && resp == nil {
		// only return an error if we don't have a response, otherwise
		// we want to insert that statuscode into the db.
		return nil, err
	}

	output <- fetchedRepo{resp.StatusCode, request.repoid, request.reponame, resp, repo}
	return resp, nil
}

func fetchRepos(ctx context.Context, wg *sync.WaitGroup, cred config.GitHubCredentials,
	repos chan fetchRequest, output chan fetchedRepo) {
	defer wg.Done()
	client := createGithubClient(ctx, cred)
Loop:
	for {
		select {
		case <-ctx.Done():
			break Loop
		case repo, ok := <-repos:
			if !ok {
				break Loop
			}

			fmt.Printf("Fetching '%s'\n", repo.reponame)
			resp, err := fetchRepo(ctx, client, repo, output)
			if err != nil {
				// If we got an error, and the context has been cancelled don't sweat it
				select {
				case <-ctx.Done():
					break Loop
				default:
				}

				// TODO: better error handling
				fmt.Printf("Error fetching '%s': %s\n", repo.reponame, err.Error())

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

func importJSONFile(ctx context.Context, db *githubanalysis.Database, filename string) error {
	// TODO: extract to common case? since we aren't actually parsing githubarchive files
	scanner, err := githubarchive.NewScanner(filename)
	if err != nil {
		return err
	}

	// filename is like /mnt/data/crawl/github_repos_1501872609.json.gz, get the timestamp
	// from the file and use for inserting records
	tokens := strings.Split(filename, "_")
	timestamp, err := strconv.ParseInt(strings.Split(tokens[len(tokens)-1], ".")[0], 10, 64)
	if err != nil {
		return err
	}
	tm := time.Unix(timestamp, 0)
	statuscode := 200
	defer scanner.Close()

	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}
		repo := new(github.Repository)
		err = json.Unmarshal(scanner.Bytes(), repo)
		if err != nil {
			return err
		}

		err := db.InsertRepo(&statuscode, &tm, repo, true)
		if err != nil {
			return err
		}
	}

	return nil
}

func importJSONFiles(ctx context.Context, db *githubanalysis.Database, pathname string) error {
	files, err := ioutil.ReadDir(pathname)
	if err != nil {
		return err
	}

	for _, file := range files {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		if !file.IsDir() && strings.HasSuffix(file.Name(), "json.gz") {
			filename := path.Join(pathname, file.Name())
			fmt.Printf("Importing: %s\n", filename)
			if err = importJSONFile(ctx, db, filename); err != nil {
				return err
			}
		}
	}
	return nil
}

func queueRepos(ctx context.Context, db *githubanalysis.Database,
	filename string, repos chan fetchRequest, refetch bool) error {
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
			repoid, err := strconv.ParseInt(tokens[1], 10, 64)
			if err != nil {
				return err
			}
			if !refetch {
				hasrepo, err := db.HasRepo(repoid)
				if err != nil {
					fmt.Printf("Failed to query repo status '%s': %s", tokens[2], err.Error())
				}

				if hasrepo {
					// fmt.Printf("Skipping %s\n", tokens[2])
					continue
				}
			}

			select {
			case <-ctx.Done():
				return nil
			case repos <- fetchRequest{repoid, tokens[2]}:
			}
		}

	}
	return nil
}

func main() {
	importjson := flag.Bool("importjson", false, "re-insert json data")
	refetch := flag.Bool("refetch", false, "Refetch repos that already exist in the database")
	jsonpath := flag.String("jsonpath", "", "location of json files")
	filename := flag.String("filename", "", "Filename to process")
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

	if *importjson {
		err := importJSONFiles(ctx, db, *jsonpath)
		if err != nil {
			fmt.Printf("Error reading json %s", err.Error())
		}
		os.Exit(0)
	}

	repos := make(chan fetchRequest, 100)
	output := make(chan fetchedRepo)

	// create a goroutine per credential to handle making api requests
	var wg sync.WaitGroup
	for _, cred := range cfg.GitHubCredentials {
		wg.Add(1)
		go fetchRepos(ctx, &wg, cred, repos, output)
	}

	// create a single goroutine for writing results to db/disk
	var outputWG sync.WaitGroup
	outputWG.Add(1)
	go writeRepo(ctx, &outputWG, output, db, *jsonpath)

	err = queueRepos(ctx, db, *filename, repos, *refetch)
	if err != nil {
		panic(err)
	}

	close(repos)
	wg.Wait()
	close(output)
	outputWG.Wait()
}
