FROM golang:1.24-alpine AS generate
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
WORKDIR /src
COPY sqlc.yaml .
COPY sql/ sql/
RUN sqlc generate

FROM golang:1.24-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=generate /src/internal/db/ internal/db/
RUN CGO_ENABLED=0 go build -o /server ./cmd/server
RUN CGO_ENABLED=0 go build -o /migrate ./cmd/migrate

FROM alpine:3.19
WORKDIR /app
COPY --from=build /server /app/server
COPY --from=build /migrate /app/migrate
EXPOSE 8080
CMD ["/app/server"]
