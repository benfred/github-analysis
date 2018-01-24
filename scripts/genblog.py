import logging
import os

import plot


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)

    import toml
    config_file = os.path.join(os.path.dirname(__file__), "..", "config.toml")
    config = toml.load(open(config_file))

    logging.basicConfig(level=logging.DEBUG)
    data = plot.load_data(config['githubarchivepath'])

    plot.generate_readme(data,
                         image_path="/Users/ben/code/blog/images/github/language-popularity/",
                         image_url_path="/images/github/language-popularity/",
                         template_file="/Users/ben/code/blog/_includes/langpopularity_template.html",  # noqa
                         output_file="/Users/ben/code/blog/_includes/langpopularity.html")

    plot.generate_graphs(data, path="/Users/ben/code/blog/images/github/language-popularity/")
