## How to run the simulator

### Initialize postgres database
```
docker run --name my-postgres -e POSTGRES_PASSWORD=secret -p 5432:5432 -d postgres:17-alpine
```

### You can connect to database with password "secret"
```
psql -h localhost -p 5432 -U postgres -d postgres
```

### Now run delivery service simulator
```
go run ./cmd/delivery/main.go
```

#### delivery service should be run in mulitple instances by putting delivery services behind a nginx proxy you can distribute worker processes over multiple instances

### Also run 3pl dumb service too
```
go run ./cmd/3pl/main.go
```

### At the end generate simulated shipment requests via seeder script
```
go run ./cmd/seeder/main.go
```