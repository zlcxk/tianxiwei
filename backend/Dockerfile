FROM golang:1.26-alpine AS build
WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/worker ./cmd/worker

FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
 && addgroup -S xiwei \
 && adduser -S -G xiwei -h /app xiwei

WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker

USER xiwei
EXPOSE 8080
ENTRYPOINT ["/app/api"]
