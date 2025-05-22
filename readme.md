# Gator -- CLI-based RSS Aggregator
- Follow RSS feeds
- Browse posts from feeds you follow
- Open rss posts in browser 

## REQUIREMENTS
(Note: developed and tested on Mac only)
- Install Go toolchain (version 1.23+)
- Install Postgres v15 or later:

## USAGE
Dispaly help commands:
```
gator help
```
Regsiter user:
```
gator register dan
```
Add and follow an RSS feed:
```
gator addfeed "Tech Crunch" "https://techcrunch.com/feed/"
```
In a standalone terminal, aggregate the feed posts to the database:
```
gator agg duration (e.g. 1m | 1hr | 2hr )
```
Browse posts that have been aggregated:
```
gator browse limit (default limit is 2)
```
Open a post in the browser:
```
gator openpost id (id is to the left of the post url in brackets)
```
Reset database (warning: destructive!):
```
gator reset
```


## SETUP
1. Install Postgres
mac OS with brew
```
brew install postgres@15
```

Linux/WSL (Debian)
```
sudo apt update
sudo apt install postgresql postgresql-contrib
```
Be sure postgres is in your PATH:
```
export PATH="/usr/local/opt/postgresql@15/bin:$PATH"    
```
Ensure the installation worked:
```
psql --version
```

(Linux Only) Upgrade postgres password:
```
sudo passwd postgre
```
2. Start the postgres server in the background
macOS: 
```
brew services start postgresql@15
```
Linux: 
```
sudo service postgresql start
```

3. Connect to the server:
macOS: 
```
psql postgres
```
Linux: 
```
sudo -u postgres psql
```

4. Create the database with the name you will put into your config file (see below):
```
postgres=# CREATE DATABASE gator;
```

(Linux Only) Connect to database and set user password (for simplicity, 'postgres' in example):
```
postgres=# \c gator
gator=# ALTER USER postgres PASSWORD 'postgres';
```
5. Type exit to leave psql shell.

6. Create file name .gatorconfig.json with connection string
```
{
 "db_url": "postgres://user_name:@localhost:5432/db_name?sslmode=disable",
 "current_user_name": "current_user_name"
}
```

7. Install Goose (I recommend installing it using ```go install```:
```
go install github.com/pressly/goose/v3/cmd/goose@latest
```

From the sqlc/schema directory, run
```
goose postgres <connection_string> up
```
Where <connection_string> is the db_url from the config file.

8. Install SQLC using go install:
```
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
```
9. Run ```sqlc generate``` from the main project directory

10. Install Google's uuid package
 ```
go get github.com/google/uuid
```

11. Import a postgres driver
```
go get github.com/lib/pq
```

12. Install app using go install from project root
```
go install 
```
