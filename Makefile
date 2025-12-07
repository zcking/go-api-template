BUF_VERSION:=v1.32.2
SWAGGER_UI_VERSION:=v4.15.5

run:
	go run cmd/server/main.go

generate:
	go run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION) generate

lint:
	go run github.com/bufbuild/buf/cmd/buf@$(BUF_VERSION) breaking --against 'https://github.com/zcking/go-api-template.git#branch=main'

# Docker commands
docker:
	docker build -t go-api-template .

docker/run:
	docker run --rm -it -p 8080:8080 -p 8081:8081 go-api-template

# Docker Compose commands
compose/up:
	docker-compose up --build

compose/up/api:
	docker-compose up --build api

compose/down:
	docker-compose down

compose/logs:
	docker-compose logs -f

# Migration commands
migrate/create:
	@read -p "Enter migration name: " name; \
	migrate create -ext sql -dir migrations $$name

migrate/up:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/go_api_template?sslmode=disable" up

migrate/down:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/go_api_template?sslmode=disable" down

migrate/version:
	migrate -path migrations -database "postgres://postgres:postgres@localhost:5432/go_api_template?sslmode=disable" version