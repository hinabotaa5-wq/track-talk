FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /api ./cmd/api \
 && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /importmajors ./cmd/importmajors \
 && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /importcompetitions ./cmd/importcompetitions

FROM alpine:3.21
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /api /app/api
COPY --from=build /importmajors /app/importmajors
COPY --from=build /importcompetitions /app/importcompetitions
EXPOSE 8080
ENV API_ADDR=:8080
CMD ["/app/api"]
