FROM golang:1.26.1-alpine AS build
WORKDIR /app
COPY bfa-go/go.mod bfa-go/go.sum* ./
RUN go mod download
COPY bfa-go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/bfa ./cmd/server

FROM alpine:3.22
WORKDIR /app
COPY --from=build /bin/bfa /usr/local/bin/bfa
EXPOSE 8080
ENTRYPOINT ["bfa"]

