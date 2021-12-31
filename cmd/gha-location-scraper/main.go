package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	githubanalysis "github.com/benfred/github-analysis"
	"github.com/benfred/github-analysis/config"

	"googlemaps.github.io/maps"
)

// InsertLocation inserts a google maps request into the db
func InsertLocation(conn *githubanalysis.Database, location string, fetchtime time.Time, results []maps.GeocodingResult) error {
	sql := `INSERT INTO locations (location, data, fetched) VALUES ($1, $2, $3) ON CONFLICT(location) DO UPDATE SET data = $2, fetched=$3`
	data, err := json.Marshal(results)
	if err != nil {
		return err
	}
	_, err = conn.Exec(sql, location, data, fetchtime)
	return err
}

// HasLocation returns if the location has already been fetched
func HasLocation(conn *githubanalysis.Database, location string) (bool, error) {
	// TODO: this doesn't seem all that good
	rows, err := conn.Query("SELECT fetched from locations where location=$1 and fetched is not null", location)
	if err != nil {
		return false, err
	}

	defer rows.Close()
	for rows.Next() {
		var fetched time.Time
		if err := rows.Scan(&fetched); err != nil {
			return false, err
		}
		return true, nil
	}

	return false, nil
}

func fetchLocation(client *maps.Client, db *githubanalysis.Database, location string) error {
	results, err := client.Geocode(context.Background(), &maps.GeocodingRequest{Address: location})
	if err != nil {
		fmt.Printf("failed to geocode '%s': %s", location, err.Error())
		return err
	}

	fmt.Printf("Found %d results\n", len(results))
	for _, result := range results {
		fmt.Printf("formatted %s\n", result.FormattedAddress)
		for _, component := range result.AddressComponents {
			fmt.Printf("%s\n", component.LongName)
			fmt.Printf("%s\n", component.ShortName)
			for _, t := range component.Types {
				fmt.Printf("Type: %s\n", t)
			}
			fmt.Printf("-\n\n")
		}
	}

	return InsertLocation(db, location, time.Now(), results)
}

func fetchLocations(conn *githubanalysis.Database, client *maps.Client) error {
	sql := `select location, count(*) from users where location is not null group by location order by (count(*), sum(followers)) desc`

	rows, err := conn.Query(sql)
	if err != nil {
		return err
	}

	defer rows.Close()

	errs := 0

	for rows.Next() {
		var location string
		var count int
		if err := rows.Scan(&location, &count); err != nil {
			return err
		}

		skip, err := HasLocation(conn, location)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		fmt.Printf("Location %s Count %d\n", location, count)
		err = fetchLocation(client, conn, location)
		if err != nil {
			errs += 1
			if errs >= 500 {
				return err
			}
		}
	}
	return nil
}

func main() {
	cfg := config.Read("config.toml")

	client, err := maps.NewClient(maps.WithAPIKey(cfg.GoogleMapsKey))
	if err != nil {
		log.Fatalf("fatal error: %s", err)
	}

	db, err := githubanalysis.Connect(cfg)
	if err != nil {
		log.Fatal(err)
	}

	err = fetchLocations(db, client)
	if err != nil {
		log.Fatal(err)
	}

}
