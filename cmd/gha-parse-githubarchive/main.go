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
)

func analyzeDay(pathname string) error {
	hours, err := ioutil.ReadDir(pathname)
	if err != nil {
		return fmt.Errorf("Failed to read '%s': %s", pathname, err.Error())
	}

	outputfilename := path.Join(pathname, "parsed_events.tsv")
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

				forkID := ""
				forkName := ""
				if event.Type == "ForkEvent" {
					id, name := githubarchive.ParseForkEvent(event.RepoName, it.Bytes())
					forkID = fmt.Sprintf("%d", id)
					forkName = name
				}

				fmt.Fprintf(output, "%s\t%d\t%s\t%s\t%d\t%s\t%s\t%s\n",
					event.Type,
					event.RepoID,
					event.RepoName,
					event.RepoLanguage,
					event.UserID,
					event.UserName,
					forkID,
					forkName)
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
