package githubarchive

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"time"
)

// MaybeDownloadFile checks if the githubarchive file is missing, and if so downloads it
func MaybeDownloadFile(basedir string, year int, month int, day int, hour int, dryrun bool) error {
	dir := path.Join(basedir, fmt.Sprintf("%04d", year), fmt.Sprintf("%02d", month), fmt.Sprintf("%02d", day))

	if stat, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			// try creating the dir
			if err := os.MkdirAll(dir, os.ModePerm); err != nil {
				fmt.Printf("Failed to create '%s'\n", dir)
				return err
			}
		} else {
			return err
		}
	} else if !stat.IsDir() {
		return fmt.Errorf("'%s' is not a directory", dir)
	}

	filename := path.Join(dir, fmt.Sprintf("%d.json.gz", hour))
	if _, err := os.Stat(filename); err != nil {
		if os.IsNotExist(err) {
			url := fmt.Sprintf("http://data.githubarchive.org/%04d-%02d-%02d-%d.json.gz", year, month, day, hour)
			if dryrun {
				fmt.Printf("dry-run: downloading %s\n", url)
				return nil
			}

			resp, err := http.Get(url)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			if resp.StatusCode != 200 {
				return fmt.Errorf("Failed to download '%s' -  %s", url, resp.Status)
			}

			f, err := os.Create(filename)
			if err != nil {
				return err
			}
			defer f.Close()

			bytes, err := io.Copy(f, resp.Body)
			if err != nil {
				return err
			}
			fmt.Printf("Downloaded %d bytes to '%s'\n", bytes, filename)
		} else {
			return err
		}
	}

	return nil
}

func daysInMonth(year int, m time.Month) int {
	return time.Date(year, m+1, 0, 0, 0, 0, 0, time.UTC).Day()
}

// DownloadFiles copies githubarchive files locally
func DownloadFiles(pathname string) error {
	now := time.Now()

	fmt.Printf("year %d month %d day %d hour %d\n", now.Year(), now.Month(), now.Day(), now.Hour())
	// for year := 2012; year <= now.Year(); year++ {
	for year := now.Year(); year >= 2011; year-- {
		endMonth := time.December
		if year == now.Year() {
			endMonth = now.Month()
		}

		for month := time.January; month <= endMonth; month++ {
			endDay := daysInMonth(year, month)
			if year == now.Year() && month == now.Month() {
				endDay = now.Day() - 1
			}

			for day := 1; day <= endDay; day++ {
				for hour := 0; hour < 24; hour++ {
					MaybeDownloadFile(pathname, year, int(month), day, hour, false)
				}
			}
		}
	}

	return nil
}
