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

/* Random sql queries for locations

// Groups all place_ids together
select data::json -> 0 ->> 'place_id' as place_id, count(*) from locations group by place_id order by count(*) desc;

// Groups all locations with formatted_address/placeid together. Seems like formatted_address is as good an id as place_id, and legiglbe?
select data::json -> 0 ->> 'formatted_address' as address, data::json -> 0 ->> 'place_id' as place_id, count(*) from locations group by (place_id, address) order by count(*) desc;

// Returns location strings for a placeid
select location from locations where data @> '[{"place_id": "ChIJ-RRZyGOvlVQR8-ORLBHVVoQ"}]';

// Returns users/sumfollowers grouped by formatted address
select data::json -> 0 ->> 'formatted_address' as address, sum(followers), count(*) from users inner join locations on locations.location = users.location where followers is not null group by address order by count(*) desc;

// Returns the top companies in vancouver (resolved addresses), sorted by # of employees
select data::json -> 0 ->> 'formatted_address' as address, count(*), sum(followers), company from locations inner join users on locations.location = users.location where followers is not null and data::json -> 0 ->> 'formatted_address'='Vancouver, BC, Canada' group by (address, company) order by (count(*), sum(followers)) desc;

// Returns the top locations, with summed counts/followers - along with lat/lon addresses.
select distinct address, sum, c.count, data::json -> 0 -> 'geometry' -> 'location' ->> 'lat' as lat, data::json -> 0 -> 'geometry' -> 'location' ->> 'lng' as lng  from (select data::json -> 0 ->> 'formatted_address' as address, sum(followers), count(*) from users inner join locations on locations.location = users.location where followers is not null group by address) as c inner join locations on c.address = data::json -> 0 ->> 'formatted_address' order by c.count desc limit 50;

// Returns all the ambigious locations.
select location from locations where json_array_length(data::json) != 1;

// TODO: Don't think 90 users are in Earth, Texas
 Earth, TX 79031, USA                                                                  |   7801 |    90
 // Or 50 Users in Internet, Laos
 "Internet, 13, Pakse, Laos",15.1219947,105.8021808,1135,50
 // or 104 here:
 "533 S Rockford Ave, Tulsa, OK 74120, USA",36.153315,-95.971261,4426,104


 select user.login, location, data::json -> 0 -> 'address_components' -> (json_array_length(data::json -> 0 -> 'address_components') - 1) ->> 'long_name' from users inner join locations on locations.location = user.location where user.location like '%Vancouver%';


 // This query is a bit of a beast, but pulls out country units sort of ok
select d.location, d.component->>'long_name' as country, d.component->>'short_name' as country_Code from (select c.location, addresses-> json_array_length(c.addresses) - 1 as component from (select location, data::json -> 0 -> 'address_components' as addresses from locations) as c) as d where  ((d.component->>'types')::jsonb ? 'country') order by country;

// Again a bit of a best, but groups users together by country decently well (except for poland/ukraine for some reason - also postal codes etc)
// problem in ukraine is country is 2nd to last instead of last {"types": ["country", "political"], "long_name": "Ukraine", "short_name": "UA"}, {"types": ["postal_code"], "long_name": "02000", "short_name": "02000"}
select sum, count, country ->> 'short_name' as country_code, country ->> 'long_name' as country from (select count(*), sum(users.followers), data::jsonb -> 0 -> 'address_components' -> (json_array_length(data::json -> 0 -> 'address_components') - 1)  as country from users inner join locations on locations.location = users.location group by country order by sum(users.followers) desc) as countries where countries.country -> 'types' ? 'country' order by sum desc;

// Groups users by country, solving issue with postal codes in the ukraine:
select sum(users.followers), count(1), country_map.country from users inner join (select location, array_to_json(array_agg(j.*)) -> 0 ->> 'long_name' as country from locations, json_array_elements(locations.data::json -> 0 -> 'address_components') j where (j -> 'types' ->> 0 = 'country') group by locations.location) as country_map on country_map.location = users.location group by country_map.country order by count(*) desc;

 // TODO: @heroku in vancouver doesn't seem to aggregate properly.
 // TODO: need to company-code the organization strings.

// Wowsers. Just need to generate some graphs here. This might not actually be all that hard!

So a post on 'do you need to move to the valley' is pretty much done here, just need to collect some data
*/

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
                        errs += 1;
                        if (errs >= 500) {
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
