Watcher
=======

This is a Go program for repeatedly running commands and
recording the output, then piping it into other commands
as a form of analysis. Commands are run in two categories;
"watched" commands, which take no input, and "analysis"
commands, which receive the output of other commands
as input.

For instance, watching a "df" command would be one way
of monitoring disk usage (although of course there are
ready-made tools for that), watching an "ls" command
would be a way of monitoring the contents of a particular
directory. A ping- or curl-like command could be watched
in order to watch the response time of a particular system
or the contents of a web page.

The fact that the output of commands is stored means
that analyses can be applied retroactively.

Dependencies
============

The program uses a Postgres database for orchestration
and storage. SQL files to set up the database are
included in a format suitable for use with pg-migrator.

The database is specified by giving as a flag the
filename of a YAML file of the following format:

    host: your-database-host.example.com
    port: 5432
    database: your-database-name
    user: your-database-user
    password: your-database-password

This file must have a name ending in ".secret.yaml", and
it must have permissions no more liberal than 0700.

The program also requires a config file that specifies
what commands to execute. This is also a YAML file;
see the examples directory for an example.

Legal stuff
===========

I (@steinarvk) hold the copyright on this code. It is not
associated with any employer of mine, past or present.

The code is made available for use under the MIT license;
see the LICENSE file for details.
