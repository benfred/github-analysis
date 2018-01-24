""" Quick script to extract repo/language from a ghtorrent database dump """
import csv
import logging
import os
import toml


if __name__ == "__main__":
    config_file = os.path.join(os.path.dirname(__file__), "..", "config.toml")
    config = toml.load(open(config_file))

    logging.basicConfig(level=logging.DEBUG)
    log = logging.getLogger("extract_ghtorrent")

    # we need to take some care here to read this csv in properly, the
    # format is a mysql CSV, which isn't handled well out of the box
    # by most csv readeres (and appears to be basically impossible to get
    # the built in go csv reader to parse, meaning I'm dropping back to
    # to python to do this).

    # alo projects can contain embedded carriage returns (/r) in the description
    # which need filtered out before passing to csv.reader
    projects_file = os.path.join(config["ghtorrentpath"], "projects.csv")
    reader = csv.reader((l.replace('\r', '') for l in open(projects_file)),
                        doublequote=False,
                        escapechar="\\",
                        quotechar='"',
                        skipinitialspace=True)

    output_file = os.path.join(config["githubarchivepath"], "ghtorrents_reponame_language.tsv")

    with open(output_file, "w") as o:
        for i, line in enumerate(reader):
            if len(line) != 11:
                # this shouldn't be happening any more, but lets just check anyways
                log.warning("error on line %i", i)
                log.info("problem line is '%s'", line)
                continue

            language = line[5]
            repo = line[1].replace("https://api.github.com/repos/", "")

            # skip nulls
            if language in {'\\N', 'N'}:
                continue

            # write out as tab-separated
            o.write(repo + "\t" + language + "\n")
