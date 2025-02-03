# db

## Create migration file
```
./bin/migrate create -ext sql -dir db/migrations <file_name>
```

## Up
```
./bin/migrate -path db/migrations -database "${DB_URL}" up
```

golang-migrate CLI README:
https://github.com/golang-migrate/migrate/tree/master/cmd/migrate
