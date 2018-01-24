package githubarchive

import (
	"fmt"
	"io/ioutil"
	"path"
)

// FindDayPaths returns the locations of all day like things in a subdir hierarchy
func FindDayPaths(pathname string) ([]string, error) {
	results := []string{}

	years, err := ioutil.ReadDir(pathname)
	if err != nil {
		return nil, fmt.Errorf("Failed to open '%s': %s", pathname, err.Error())
	}
	for _, year := range years {
		if year.IsDir() {
			yearpath := path.Join(pathname, year.Name())
			months, err := ioutil.ReadDir(yearpath)
			if err != nil {
				return nil, fmt.Errorf("Failed to open '%s': %s", yearpath, err.Error())
			}

			for _, month := range months {
				if month.IsDir() {
					monthpath := path.Join(yearpath, month.Name())
					days, err := ioutil.ReadDir(monthpath)
					if err != nil {
						return nil, fmt.Errorf("Failed to read '%s': %s", monthpath, err.Error())
					}

					for _, day := range days {
						if day.IsDir() {
							results = append(results, path.Join(monthpath, day.Name()))
						}
					}
				}
			}
		}
	}
	return results, nil
}
