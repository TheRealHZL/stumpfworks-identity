FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /sw-badge-server ./cmd/server
FROM alpine:3.22
RUN addgroup -S swbadge && adduser -S -G swbadge swbadge && mkdir -p /var/lib/stumpfworks-badge && chown swbadge:swbadge /var/lib/stumpfworks-badge
COPY --from=build /sw-badge-server /usr/local/bin/
USER swbadge
EXPOSE 8080
ENTRYPOINT ["sw-badge-server"]
