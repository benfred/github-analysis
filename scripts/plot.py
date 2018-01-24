""" Code to generate all the plots used by the README and in the original blog post """
from collections import defaultdict, namedtuple
import datetime
import logging
import os
import time

import jinja2
import matplotlib.dates as mdates
import matplotlib.pyplot as plt
import numpy
import scipy.signal  # noqa
import seaborn as sns

log = logging.getLogger("gha")

try:
    from urllib.request import pathname2url
except ImportError:
    from urllib import pathname2url

COUNT_FILENAME = "language_mau.txt"
MAU_FILENAME = "mau.txt"

# define a custom colour palette to draw from (d3.schemeCategory10 - to be consistent with other
# posts I've written)
COLOURS = ["#1f77b4", "#ff7f0e", "#2ca02c", "#d62728", "#9467bd", "#8c564b", "#e377c2", "#7f7f7f",
           "#bcbd22", "#17becf"]

# use a consistent mapping of language->colour across graphs generated here.
COLOUR_MAP = defaultdict(lambda: COLOURS[len(COLOUR_MAP) % len(COLOURS)])


def load_data(basedir):
    now = datetime.date.fromtimestamp(time.time() - 86400)
    languages = defaultdict(list)
    mau = {}
    for year in range(2011, now.year + 1):
        for month in range(1, now.month if year == now.year else 13):
            mau_file = os.path.join(basedir, "%04i" % year, "%02i" % month, MAU_FILENAME)
            try:
                total_mau = int(open(mau_file).read().split()[0])
            except Exception as e:
                log.info("Skipping %s: %s", mau_file, e)
                continue

            if total_mau <= 1000:
                log.info("Skipping %s: insufficient mau of %i", mau_file, total_mau)
                continue

            date = datetime.datetime(year, month, 28)
            mau[date] = total_mau
            # hack: insert mau counts into language dict to easily generate mau graph
            languages['MAU'].append((date, int(total_mau)))

            filename = os.path.join(basedir, "%04i" % year, "%02i" % month, COUNT_FILENAME)
            if not os.path.exists(filename):
                log.info("skipping non-existant file '%s'", filename)
                continue

            seen = set()
            for line in open(filename):
                tokens = line.split()
                count = int(tokens[0])
                language = " ".join(tokens[1:])

                if language in seen:
                    log.warning("Duplicate language '%s' in file '%s'", language, filename)
                    continue
                seen.add(language)
                languages[language].append((date, int(count)))

    ret = []
    for language, v in languages.items():
        v.sort()
        counts = [float(count) for _, count in v]
        dates = [d for d, _ in v]

        # if we don't have a full year, back off
        if len(counts) <= 12:
            log.info("Skipping language %s: don't have a full years worth of data", language)
            continue

        normalized = [c / mau[d] for c, d in zip(counts, dates)]

        # apply a light savgol filter to smooth out some of the noise
        normalized = scipy.signal.savgol_filter(normalized, 7, 1)
        counts = scipy.signal.savgol_filter(counts, 7, 1)

        ret.append((language, dates, counts, normalized))

    ret.sort(key=lambda x: -x[2][-1])
    return ret


def generate_stackplot(items_top, items_bottom, filename,
                       output_path="images",
                       adjustments=None):
    sns.set()
    adjustments = adjustments or {}

    fig, axarr = plt.subplots(2, sharex=True)

    for language, dates, counts, normalized in items_top:
        ax = axarr[0]
        x = dates
        y = [100 * n for n in normalized]

        ax.plot(x, y, label=language, color=COLOUR_MAP[language])
        ax.text(x[-1], y[-1] + adjustments.get(language, 0), language, fontsize=8)
        ax.set_ylim(bottom=0)
        ax.get_yaxis().get_major_formatter().set_scientific(False)

    for language, dates, counts, normalized in items_bottom:
        ax = axarr[1]
        x = dates
        y = [100 * n for n in normalized]

        ax.plot(x, y, label=language, color=COLOUR_MAP[language])
        ax.text(x[-1], y[-1] + adjustments.get(language, 0), language, fontsize=8)

    years = mdates.YearLocator()   # every year
    months = mdates.MonthLocator()  # every month
    yearsFmt = mdates.DateFormatter('%Y')
    ax.xaxis.set_major_locator(years)
    ax.xaxis.set_major_formatter(yearsFmt)
    ax.xaxis.set_minor_locator(months)
    ax.format_xdata = mdates.DateFormatter('%Y-%m-%d')

    ax.get_yaxis().get_major_formatter().set_scientific(False)
    ax.set_ylim(bottom=0)
    plt.ylabel('Percentage of MAU')
    fig.autofmt_xdate()

    # pad out the graph so we can display language names
    ax.set_xlim(right=datetime.date.today() + datetime.timedelta(365))
    plt.savefig(filename, dpi=600, bbox_inches='tight')


def generate_plot(items, filename, ylabel="Percentage of MAU", normalize=True,
                  adjustments=None):
    sns.set()
    adjustments = adjustments or {}

    fig, ax = plt.subplots()

    for language, dates, counts, normalized in items:
        if normalize:
            x = dates
            y = [100 * n for n in normalized]
        else:
            x = dates
            y = counts

        ax.plot(x, y, label=language, color=COLOUR_MAP[language])
        ax.text(x[-1], y[-1] + adjustments.get(language, 0),
                language.replace(" ", "\n"), fontsize=8)

    years = mdates.YearLocator()   # every year
    months = mdates.MonthLocator()  # every month
    yearsFmt = mdates.DateFormatter('%Y')
    ax.xaxis.set_major_locator(years)
    ax.xaxis.set_major_formatter(yearsFmt)
    ax.xaxis.set_minor_locator(months)
    ax.format_xdata = mdates.DateFormatter('%Y-%m-%d')
    ax.get_yaxis().get_major_formatter().set_scientific(False)

    plt.ylabel(ylabel)
    fig.autofmt_xdate()
    ax.set_ylim(bottom=0)

    # pad out the graph so we can display language names
    ax.set_xlim(right=datetime.date.today() + datetime.timedelta(365))
    plt.savefig(filename, dpi=600, bbox_inches='tight')


def geometric_mean(vals):
    return numpy.exp(sum(numpy.log(vals) / len(vals)))


def sparkline(x, y, filename, figsize=(2.5, .4), fill=True):
    fig, ax = plt.subplots(figsize=figsize)

    if fill:
        ax.fill_between(x, y, numpy.zeros(len(y)), alpha=0.1)
    else:
        ax.plot(x, numpy.zeros(len(x)), color='lightgrey')

    ax.plot(x, y)

    # hack, if we just set top to the max - we lose sense of scale amongst
    # different plots. If however we fully normalize, non-popular languages
    # look like a flat line along the 0-axis. So mostly set top to the max
    # but keep like 20% of the scale (geometrically) in here for a sense of
    # scale
    top = max(y)
    top = geometric_mean([top, top, top, 1])
    ax.set_ylim(bottom=0, top=top)

    for k, v in ax.spines.items():
        v.set_visible(False)
    ax.set_xticks([])
    ax.set_yticks([])

    fig.subplots_adjust(top=0.95, bottom=0.05, left=0.0, right=1)
    fig.savefig(filename, transparent=True)
    plt.close(fig)


def generate_readme(data, count=25,
                    image_path="images/",
                    image_url_path="./images/",
                    template_file="README_template.md", output_file="README.md"):
    # filter out some non-programming language things
    data = [d for d in data if d[0] not in {'Unknown', 'Missing', 'None', 'MAU'}]

    # This might be moderately controversial: but I dont' consider any of these actual programming
    # languages
    data = [d for d in data if d[0] not in {'CSS', 'TeX', 'HTML', 'XML', 'Makefile', 'CMake', 'Vue',
                                            'Vim', 'VimL', 'Vim script', 'Emacs', 'Emacs Lisp',
                                            'Arduino', 'Visual', 'Batchfile', 'XSLT'}]

    Language = namedtuple("Langage", ['language', 'order', 'mau', 'trendfile', 'rank'])

    languages = []

    for rank, (language, x, _, y) in enumerate(data[:count]):
        COLOUR_MAP[language]
        filename = os.path.join(image_path, language + "_sparkline.svg")
        url_filename = os.path.join(image_url_path, language + "_sparkline.svg")

        sparkline(x, y, filename, fill=True)

        now = y[-1]
        languages.append(Language(language,
                                  y[-1],
                                  "%.2f%%" % (now * 100),
                                  pathname2url(url_filename), rank+1))
        print(rank, language, 100 * y[-1])
    COLOUR_MAP['MAU'] = COLOURS[0]

    generated_at = datetime.datetime.now().strftime("%B %d %Y")
    preamble = "This File was Autogenerated from %s on %s. Do not modify this file"
    preamble = preamble % (template_file, generated_at)

    template = jinja2.Template(open(template_file).read())
    with open(output_file, "w") as output:
        output.write(template.render(languages=languages,
                                     preamble=preamble,
                                     generated_at=generated_at
                                     ))
    return languages


def generate_graphs(data, path="images"):
    def get_data(languages):
        return [x for x in data if x[0] in languages]

    generate_plot(get_data({'MAU'}),
                  os.path.join(path, "mau.svg"),
                  normalize=False, ylabel="Monthly Active Users")

    generate_plot(get_data({'JavaScript', 'Python', 'Java', 'C++', 'C', 'C#'}),
                  os.path.join(path, "major.svg"))

    generate_stackplot(get_data({'TypeScript', 'CoffeeScript'}),
                       get_data({'Objective-C', 'Swift'}),
                       os.path.join(path, "cannibals.svg"),
                       adjustments={'Swift': -0.2, 'Objective-C': .0})

    generate_plot(get_data({'Go', 'TypeScript', 'Kotlin', 'Rust'}),
                  os.path.join(path, "newthing.svg"))

    generate_plot(get_data({'Ruby', 'PHP', 'Objective-C', 'CoffeeScript', 'Perl'}),
                  os.path.join(path, "oldthing.svg"),
                  adjustments={'CoffeeScript': 0.0, 'Perl': -.4})

    generate_plot(get_data({'Ruby', 'PHP', 'Objective-C', 'CoffeeScript', 'Perl'}),
                  os.path.join(path, "oldthing_u.svg"),
                  normalize=False, ylabel="Monthly Active Users", adjustments={'Perl': -3000})

    generate_plot(get_data({'R', 'Matlab', 'Jupyter Notebook'}),
                  os.path.join(path, "scientific.svg"))

    generate_plot(get_data({'Scala', 'Haskell', 'Lisp', 'Clojure', 'Erlang', 'Elixir'}),
                  os.path.join(path, "functional.svg"))


if __name__ == "__main__":
    import toml
    config_file = os.path.join(os.path.dirname(__file__), "..", "config.toml")
    config = toml.load(open(config_file))

    logging.basicConfig(level=logging.DEBUG)
    data = load_data(config['githubarchivepath'])
    generate_readme(data)
    # generate_graphs(data)
