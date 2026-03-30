FROM golang:1.26.1-alpine AS build
WORKDIR /app
COPY mock-services/go.mod mock-services/go.sum* ./
RUN go mod download
COPY mock-services/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /bin/mock-services ./cmd/server

FROM alpine:3.22
WORKDIR /app
COPY --from=build /bin/mock-services /usr/local/bin/mock-services
EXPOSE 8081
ENTRYPOINT ["mock-services"]

