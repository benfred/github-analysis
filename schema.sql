CREATE TABLE repos
(
  id integer PRIMARY KEY,
  name varchar(200) NOT NULL,
  language varchar(50),
  description text,
  size integer NOT NULL default 0,
  stars integer NOT NULL default 0,
  forks integer NOT NULL default 0,
  topics text[],
  deleted boolean NOT NULL default false,
  parentid integer,
  ownerid integer,
  created timestamp without time zone,
  modified timestamp without time zone,
  fetched timestamp without time zone,
  statuscode integer
);

CREATE TABLE users
(
  id integer PRIMARY KEY,
  login varchar(200) NOT NULL,
  name text,
  company text,
  location text,
  bio text,
  email text,
  type text,
  followers integer,
  following integer,
  created timestamp without time zone,
  modified timestamp without time zone,
  fetched timestamp without time zone,
  statuscode integer
);

CREATE INDEX repos_name_index ON repos (name);
CREATE INDEX users_login_index ON users(login);
