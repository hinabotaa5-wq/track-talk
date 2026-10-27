FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS build
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /api ./cmd/api \
 && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /importmajors ./cmd/importmajors \
 && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /importcompetitions ./cmd/importcompetitions \
 && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o /budgetstop ./cmd/budgetstop

FROM alpine:3.24
RUN apk add --no-cache ca-certificates \
 && adduser -D -H -u 10001 app
WORKDIR /app
COPY --from=build /api /app/api
COPY --from=build /importmajors /app/importmajors
COPY --from=build /importcompetitions /app/importcompetitions
COPY --from=build /budgetstop /app/budgetstop
EXPOSE 8080
ENV API_ADDR=:8080
# このイメージは Cloud Run などの本番で動かす前提なので、ローカル以外の DB への接続を許可する。
# go run で直接起動する開発時はこの値が無く、ローカル DB 以外には接続できない。
ENV ALLOW_REMOTE_DB=1
# root では動かさない(ファイルに書かないので、書き込みの権限もいらない)。
USER app
CMD ["/app/api"]
