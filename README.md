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

### Also run 3pl dumb service too
```
go run ./cmd/3pl/main.go
```

### At the end generate simulated shipment requests via seeder script
```
go run ./cmd/seeder/main.go
```