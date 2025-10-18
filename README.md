# Go API Template

This is a template I created for a simple, clean API implementation using the following tech stack:  

- [Go](https://go.dev/) - programming language of choice
- [gRPC](https://grpc.io) - modern open source high performance Remote Procedure Call (RPC) framework
- [gRPC-gateway](https://github.com/grpc-ecosystem/grpc-gateway) - gRPC to JSON proxy generator
- [buf](https://buf.build/docs/introduction) - Protocol buffers build tool
- [DuckDB](https://duckdb.org/) - fast, in-process, analytical database

Everything but DuckDB in this list are technology choices I would consider very standard/common and versatile to create any modern API. I decided to use DuckDB as well to provide a modern, yet super simple database fabric, hence the "lite" in this project's name. 

---

## Getting Started: Docker

To build and run the Docker image, just run:  

```shell
make docker docker/run
```

You should see an output like the following:  

```
docker run --rm -it -p 8080:8080 -p 8081:8081 go-api-template
2024/06/06 01:24:05 setting up database at lite.duckdb...
2024/06/06 01:24:05 gRPC Gateway listening on http://0.0.0.0:8081
```

You can call the REST API to create a user like so:  

```shell
curl --location 'http://localhost:8081/api/v1/users' \
--header 'Content-Type: application/json' \
--header 'Accept: application/json' \
--data-raw '{
  "name": "John Doe",
  "email": "jdoe@userapi.com"
}'
```

And list users with:  

```shell
curl --location 'http://localhost:8081/api/v1/users' \
--header 'Accept: application/json'
```

## Changing Protobuf

You can change the protobuf at [proto/users/v1/users.proto](./proto/users/v1/users.proto). Then use `make generate` to generate all new stubs, which are written to the [gen/](./gen/) directory.
