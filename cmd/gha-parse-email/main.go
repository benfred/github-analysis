package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"runtime"
	"strings"
	"sync"

	"github.com/benfred/github-analysis/githubarchive"
	"github.com/buger/jsonparser"
)

type commitAuthor struct {
	email string
	name  string
}

// ParsePushCommits returns a list of author/email if the
func parsePushCommits(data []byte) ([]*commitAuthor, error) {
	var authors []*commitAuthor

	jsonparser.ArrayEach(data, func(value []byte, dataType jsonparser.ValueType, offset int, err error) {
		// TODO: error handling?
		distinct, err := jsonparser.GetBoolean(value, "distinct")
		author, err := jsonparser.GetString(value, "author", "name")
		email, err := jsonparser.GetString(value, "author", "email")
		if distinct {
			authors = append(authors, &commitAuthor{email: email, name: author})
		}
	}, "payload", "commits")
	return authors, nil
}

func analyzeDay(pathname string) error {
	hours, err := ioutil.ReadDir(pathname)
	if err != nil {
		return fmt.Errorf("Failed to read '%s': %s", pathname, err.Error())
	}

	outputfilename := path.Join(pathname, "parsed_email.tsv")
	if _, err = os.Stat(outputfilename); !os.IsNotExist(err) {
		fmt.Printf("Skipping '%s' - already exists\n", outputfilename)
		return nil
	}

	output, err := os.Create(outputfilename)
	if err != nil {
		return fmt.Errorf("Failed to open file '%s' for writing: %s", outputfilename, err.Error())
	}
	defer output.Close()

	events := 0
	for _, hour := range hours {
		if !hour.IsDir() && strings.HasSuffix(hour.Name(), "json.gz") {
			hourpath := path.Join(pathname, hour.Name())
			it, err := githubarchive.NewScanner(hourpath)
			if err != nil {
				return err
			}
			defer it.Close()

			for it.Scan() {
				events++
				event := it.Event()

				if event.Type == "PushEvent" {
					authors, err := parsePushCommits(it.Bytes())
					if err == nil && len(authors) > 0 {
						author := authors[len(authors)-1]
						tokens := strings.Split(author.email, "@")
						domain := tokens[len(tokens)-1]
						fmt.Fprintf(output, "%d\t%s\t%s\t%s\t%s\n", event.UserID, event.UserName, author.name, author.email, domain)
					}
				}
			}
		}
	}

	fmt.Printf("Finished analyzing path '%s' - %d events\n", pathname, events)
	return nil
}

func main() {
	filename := flag.String("filename", "", "Filename to process")
	pathname := flag.String("path", "", "path to process")
	flag.Parse()

	if len(*pathname) > 0 {
		dirs, err := githubarchive.FindDayPaths(*pathname)
		if err != nil {
			log.Fatal(err)
		}

		numCPUs := runtime.NumCPU()
		runtime.GOMAXPROCS(numCPUs + 1)

		var wg sync.WaitGroup

		pathChan := make(chan string, 100)

		worker := func() {
			defer wg.Done()
			for path := range pathChan {
				err := analyzeDay(path)
				if err != nil {
					fmt.Printf("Failed to process '%s': %s\n", path, err.Error())
					panic(err)
				}
			}
		}

		for i := 0; i < numCPUs; i++ {
			wg.Add(1)
			go worker()
		}

		for _, dir := range dirs {
			pathChan <- dir
		}
		close(pathChan)
		wg.Wait()

	} else if len(*filename) > 0 {
		err := analyzeDay(*filename)
		if err != nil {
			panic(err)
		}
	} else {
		flag.Usage()
	}
}
