#!/bin/bash
function usage {
    echo "This script calculates a mapping of repo:language by merging information from multiple different sourcess"
    echo "usage: calculate_repo_languages.sh <githubarchiveroot>"
    exit 1
}

if [ -z $1 ] ; then
    usage
elif [ ! -d $1 ] ; then
    echo "Directory '$1' doesn't exist!"
    usage
fi
export LC_COLLATE=C

pushd $1

# get a dump of repoid/ reponame/ language from postgres (output of the scraper), this will probably require entering a password
echo "Getting repo languages from Postgres"
psql github -c "COPY (select id, name, COALESCE(language, 'None') from repos WHERE statuscode = 200 OR language is not null) TO STDOUT WITH NULL AS ''" | sort -S 80% > crawled_languages.tsv

# Generate a map of repoid/reponame/usercount from the extracted GitHubArchive events: needed to lookup repoid for GHTorrent project
# which doesn't include this (just the reponame)
echo "Looking up repoid for GHTorrent data"
find -name parsed_events.tsv | xargs awk -F $'\t' '{print $3 "\t" $2}' | sort -u -S 80% | grep -v '\-1$' > repoids.tsv

# get repoid/reponame/language from ghtorrents, by joining with the repoids.tsv
sort -S 80% ghtorrents_reponame_language.tsv > ghtorrents_reponame_language_sorted.tsv
join -t $'\t' -1 1 -2 1 -o 2.2,1.1,1.2 ghtorrents_reponame_language_sorted.tsv repoids.tsv | sort -S 80% > ghtorrent_languages.tsv

# Join languages from Postgres with Languages from GHTorrent in a single file, preferring our own scraped data if has both
echo "Merging languages from Postgres DB and GHTorrent"
time join -t $'\t' -a 1 -a 2 -e Missing -o 0,1.2,2.2,1.3,2.3 -1 1 -2 1 crawled_languages.tsv ghtorrent_languages.tsv  |
        awk -F $'\t' '{if ($2 == "Missing") $2 = $3; if ($4 == "Missing") $4 = $5; print $1 "\t" $2 "\t" $4}' |
        grep -v '^-1' | uniq > temp_language.tsv

# join the output of that with the fork file analysis, preferring previous if given
echo "Analyzing GithubArchive for Fork Events"
find -name parsed_events.tsv | xargs grep "^ForkEvent" | awk -F $'\t' '{print $2 "\t" $3 "\t" $7 "\t" $8}' | sort -u -S 80% | grep -v '^\-1' > forkevents.tsv

echo "Getting language for fork events"
join -t $'\t' -1 1 -2 1 -o 1.3,1.4,2.3 forkevents.tsv temp_language.tsv | grep -Ev '^(-1|0|\s)' | sort -S 80% > fork_languages.tsv

echo "Joining forked languages to main language map"
time join -t $'\t' -a 1 -a 2 -e Missing -o 0,1.2,2.2,1.3,2.3 -1 1 -2 1 temp_language.tsv fork_languages.tsv  |
        awk -F $'\t' '{if ($2 == "Missing") $2 = $3; if ($4 == "Missing") $4 = $5; print $1 "\t" $2 "\t" $4}' |
        grep -v '^-1' | uniq > temp_language2.tsv

# Get language from parsed json events (only works from 2012-2015), and PR events on 2015+
echo "Analyzing GithubArchive for embedded language information"
find -name parsed_events.tsv | xargs awk -F $'\t' '{print $2 "\t" $3 "\t" $4}' | grep -v $'\t$' | sort -u -S 80% > githubarchive_language.tsv

# Finally join that again with main language map to get the final repoid/reponame/language mapping
echo "Joining extracted GitHub language info with main language map"
time join -t $'\t' -a 1 -a 2 -e Missing -o 0,1.2,2.2,1.3,2.3 -1 1 -2 1 temp_language2.tsv githubarchive_language.tsv  |
        awk -F $'\t' '{if ($2 == "Missing") $2 = $3; if ($4 == "Missing") $4 = $5; print $1 "\t" $2 "\t" $4}' |
        grep -v '^-1' | uniq > repo_languages.tsv
