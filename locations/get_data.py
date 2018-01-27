""" Quick script to extract repo/language from a ghtorrent database dump """

import csv
import logging
import os
import json
import toml
import psycopg2


def update_invalid_flag(conn, filename="invalid_locations.txt"):
    with conn.cursor() as cursor:
        for location in open(filename):
            location = location.strip()
            sql = """update locations set invalid=True where location=%s"""
            cursor.execute(sql, (location,))
    conn.commit()


def update_extracted_values(conn):
    with conn.cursor() as cursor:
        # trying to do this as part of other queries was getting kinda crazy, so instead
        # extract the country/state/county/city from the json value and store in a column
        sql = """update locations set state=map.state from (
                 select location, array_to_json(array_agg(j.*)) -> 0 ->> 'long_name' as state from locations,
                     json_array_elements(locations.data::json -> 0 -> 'address_components') j
                                     where ((j-> 'types')::jsonb ? 'administrative_area_level_1') group by locations.location) as map
                 where map.location = locations.location;"""
        cursor.execute(sql)

        sql = """update locations set city=map.city from (
                 select location, array_to_json(array_agg(j.*)) -> 0 ->> 'long_name' as city from locations,
                         json_array_elements(locations.data::json -> 0 -> 'address_components') j
                                         where ((j-> 'types')::jsonb ? 'locality') group by locations.location) as map
                 where map.location = locations.location;"""
        cursor.execute(sql)

        sql = """update locations set country=map.country from (
                 select location, array_to_json(array_agg(j.*)) -> 0 ->> 'long_name' as country from locations,
                         json_array_elements(locations.data::json -> 0 -> 'address_components') j
                                         where ((j-> 'types')::jsonb ? 'country') group by locations.location) as map
                 where map.location = locations.location;"""
        cursor.execute(sql)

        sql = """update locations set county=map.county from (
                 select location, array_to_json(array_agg(j.*)) -> 0 ->> 'long_name' as county from locations,
                         json_array_elements(locations.data::json -> 0 -> 'address_components') j
                                        where ((j-> 'types')::jsonb ? 'administrative_area_level_2') group by locations.location) as map
                 where map.location = locations.location;"""
        cursor.execute(sql)
    conn.commit()


def get_top_users(conn):
    sql = """select users.login, users.followers, users.location, map.lat, map.lng,
             users.name, map.country, users.company from users
             inner join (select location, country, invalid,
                         data::json -> 0 -> 'geometry' -> 'location' ->> 'lat' as lat,
                         data::json -> 0 -> 'geometry' -> 'location' ->> 'lng' as lng from locations) as map
             on map.location = users.location
             where followers is not null and map.lat is not null and (invalid is null or invalid != True)
             order by followers desc limit 4096;"""

    with conn.cursor() as cursor:
        cursor.execute(sql)
        rows = cursor.fetchall()

        # transpose data, return as a dictionary of {columnname: [values]}
        return dict((r.name, values) for r, values in zip(cursor.description, list(zip(*rows))))


def _remap_populations(populations):
    """ Some datasources have different country names than we expect (world bank gdp etc). Remap
    """
    # Remap some country names to fit with what we have from google maps api
    populations['Russia'] = populations['Russian Federation']
    populations['South Korea'] = populations['Korea, Rep.']
    populations['Iran'] = populations["Iran, Islamic Rep."]
    populations['Czechia'] = populations["Czech Republic"]
    populations['Egypt'] = populations["Egypt, Arab Rep."]
    populations['Hong Kong'] = populations["Hong Kong SAR, China"]
    populations['Venezuela'] = populations["Venezuela, RB"]
    populations['Slovakia'] = populations['Slovak Republic']
    populations['Macedonia (FYROM)'] = populations["Macedonia, FYR"]
    populations["Myanmar (Burma)"] = populations["Myanmar"]
    populations['Syria'] = populations['Syrian Arab Republic']
    populations["Côte d'Ivoire"] = populations["Cote d'Ivoire"]
    populations['Yemen'] = populations["Yemen, Rep."]
    populations["Democratic Republic of the Congo"] = populations["Congo, Dem. Rep."]
    populations["Republic of the Congo"] = populations["Congo, Rep."]
    populations['Kyrgyzstan'] = populations["Kyrgyz Republic"]
    populations['Laos'] = populations["Lao PDR"]
    populations['Brunei'] = populations["Brunei Darussalam"]
    populations['The Bahamas'] = populations["Bahamas, The"]
    populations['Macau'] = populations["Macao SAR, China"]
    populations['The Gambia'] = populations["Gambia, The"]
    populations['U.S. Virgin Islands'] = populations["Virgin Islands (U.S.)"]
    populations['Saint Lucia'] = populations["St. Lucia"]
    populations['Saint Kitts and Nevis'] = populations["St. Kitts and Nevis"]
    populations['North Korea'] = populations.get("Korea, Dem. People’s Rep.", 0)


def get_country_populations(filename=None):
    # read a dictionry of country: population
    filename = filename or os.path.join(os.path.dirname(__file__), "world_population_2016.csv")
    populations = dict((country, int(count))
                       for country, _, _, count in csv.reader(open(filename)))

    _remap_populations(populations)

    # taiwan is missing from here, add from wikipedia (2016)
    # https://en.wikipedia.org/wiki/Demographics_of_Taiwan
    populations['Taiwan'] = 23539816

    return populations


def get_country_gdp(filename=None):
    filename = filename or os.path.join(os.path.dirname(__file__), "API_NY.GDP.MKTP.CD_DS2_en_csv_v2.csv")
    r = csv.reader(open(filename))

    # burn through the first 4 entries
    [next(r) for x in range(4)]

    countries = {}
    for row in r:
        try:
            lastYear = float([x for x in row if x][-1])
            countries[row[0]] = lastYear
        except Exception as e:
            print(e, row)
    _remap_populations(countries)

    # from wikipedia
    countries['Taiwan'] = 571500000000

    return countries


def get_top_countries(conn):
    sql = """select country, count(*),
                sum(log(1+ users.followers)) as logfollowers,
                sum(sqrt(users.followers)) as sqrtfollowers,
                sum(followers) as sumfollowers
            from users inner join locations on locations.location = users.location
            where users.followers is not null
                and (locations.invalid is null or locations.invalid != True)
                and locations.country is not null
            group by country having count(*) > 1 order by count(*) desc"""

    with conn.cursor() as cursor:
        cursor.execute(sql)
        rows = cursor.fetchall()
        return [x.name for x in cursor.description], rows


def get_top_cities(conn):
    sql = """select country, state, city, count(*),
                sum(log(1+ users.followers)) as logfollowers,
                sum(sqrt(users.followers)) as sqrtfollowers,
                sum(followers) as sumfollowers
            from users inner join locations on locations.location = users.location
            where users.followers is not null
                and (locations.invalid is null or locations.invalid != True)
                and locations.country is not null and locations.city is not null
            group by (country, state, city) having count(*) > 9 order by count(*) desc"""

    with conn.cursor() as cursor:
        cursor.execute(sql)
        rows = cursor.fetchall()
        header = [x.name for x in cursor.description]

        # augment city view with county level data for SF Bay Area
        sql = """select 'United States', 'California', 'San Francisco Bay Area', count(*),
                    sum(log(1+ users.followers)) as logfollowers,
                    sum(sqrt(users.followers)) as sqrtfollowers,
                    sum(followers) as sumfollowers
                from users inner join locations on locations.location = users.location
                where users.followers is not null
                    and (locations.invalid is null or locations.invalid != True)
                    and locations.county in  ('San Francisco County', 'San Mateo County', 'Santa Clara County', 'Alameda County')"""
        cursor.execute(sql)
        rows.append(cursor.fetchone())

        return header, rows


def get_follower_distribution(conn):
    sql = """select country, followers, count(*) from users
    inner join locations on users.location = locations.location
    group by (country, followers) order by followers desc;"""

    with conn.cursor() as cursor:
        cursor.execute(sql)
        header = [x.name for x in cursor.description]
        return header, cursor.fetchall()


def write_country_data(conn, filename="top_countries.tsv"):
    headers, rows = get_top_countries(conn)
    populations = get_country_populations()
    country_gdp = get_country_gdp()

    with open(filename, "w") as o:
        o.write("\t".join(headers + ['population', 'gdp']) + "\n")
        for row in rows:
            pop = populations.get(row[0], 0)
            gdp = country_gdp.get(row[0], 0)
            o.write("\t".join(map(str, row + (pop, gdp))) + "\n")


def write_city_data(conn, filename="top_cities.tsv"):
    headers, rows = get_top_cities(conn)
    with open(filename, "w") as o:
        o.write("\t".join(headers) + "\n")
        for row in rows:
            o.write("\t".join(map(str, row)) + "\n")


def write_map_data():
    """ quick hack to annotate topojson world map with the country names.
    data can be found https://github.com/KoGor/Maps.GeoInfo/ """
    world = json.load(open("world-110m.json"))
    names = dict((int(k), v) for k, v in (l.split('\t') for l in open("world-country-names.tsv").read().split("\n")[1:] if l))
    for country in world['objects']['countries']['geometries']:
        name = names.get(int(country['id']), 'Unknown')
        country['properties'] = {'name': name}
    open("mapdata.jsonp", "w").write("var mapdata = " + json.dumps(world) + ";")


def get_db_conn(config_filename=None):
    config_filename = config_filename or os.path.join(os.path.dirname(__file__), "..", "config.toml")
    config = toml.load(open(config_filename))
    dbconfig = config['Database']
    return psycopg2.connect(host=dbconfig['host'], user=dbconfig['username'], password=dbconfig['password'], database=dbconfig['dbname'])


if __name__ == "__main__":
    logging.basicConfig(level=logging.DEBUG)
    log = logging.getLogger("extract_users_jsonp")

    conn = get_db_conn()

    # update state in db
    update_invalid_flag(conn)
    update_extracted_values(conn)

    data = get_top_users(conn)
    with open("top_users.jsonp", "w") as o:
        o.write("var topusers = " + json.dumps(data, indent=2) + ";")

    write_city_data(conn)
    write_country_data(conn)
