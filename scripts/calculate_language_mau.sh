#!/bin/bash
function usage {
    echo "usage: calculate_repo_user_count.sh <githubarchiveroot>"
    exit 1
}

if [ -z $1 ] ; then
    usage
elif [ ! -d $1 ] ; then
    echo "Directory '$1' doesn't exist!"
    usage
fi

export LC_COLLATE=C

for year in $1/*/; do
    echo "processing year: $year"
    for month in $year*/; do
        echo "   processing month $month"

        # figure out the number of MAU in the month
        time awk -F $'\t' '{print $6}' $month*/parsed_events.tsv  | sort -u -S 80% | wc -l > $month/mau.txt

        # extract tuples of repoid/reponame/repolanguage/userid from the events, and sort/dedupe them
        time awk -F $'\t' '{print $2 "\t" $3 "\t" $4 "\t" $6}' $month*/parsed_events.tsv | sort -u -S 80% > $month/repo_user.tsv

        # join this file agains the larger list of repo languages passed as a parameter
        time join -t $'\t' -a 1 -e Missing -o 1.3,2.3,1.4 -1 1 -2 1 $month/repo_user.tsv $1/repo_languages.tsv |
            # take the language from the event if given, otherwise use the language from language mapping, and write out language/user tuple
            awk -F $'\t' '{if ($1 == "Missing") $1 = $2; print $1 "\t" $3}' |
            # sort/deduplicate the language/user tuples for the month
            sort -S 60% -u |
            # extract the language and count the number of occurences (users) that it occurred for
            awk -F $'\t' '{print $1}' | uniq -c | sort -nr > $month/language_mau.txt
    done
done
