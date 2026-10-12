FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /api ./cmd/api && CGO_ENABLED=0 go build -o /importmajors ./cmd/importmajors

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /api /app/api
COPY --from=build /importmajors /app/importmajors
EXPOSE 8080
ENV API_ADDR=:8080
CMD ["/app/api"]
