#!/bin/bash
function usage {
    echo "This script calculates the top repos in the githubarchive by the number of users"
    echo "usage: calculate_top_repos.sh <githubarchiveroot>"
    exit 1
}

if [ -z $1 ] ; then
    usage
elif [ ! -d $1 ] ; then
    echo "Directory '$1' doesn't exist!"
    usage
fi

export LC_COLLATE=C

# print out the repoid/reponame/username from each event (assuming repoid is given)
find $1 -name parsed_events.tsv | xargs awk -F $'\t' '{if ($2 != "-1") print $2 "\t" $3 "\t" $6}' | 
    # sort and deduplicate these triples
    sort -u -S 60% | 
    # extract just the repoid/reponame
    awk -F $'\t' '{print $1 "\t" $2}' |
    # count up how many times (users) for each one, and dump to a file sorted by the number of
    # users
    uniq -c | sort -nr -S 20% > top_repos.txt
