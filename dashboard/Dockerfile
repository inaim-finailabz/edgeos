# Build from the repo root: docker build -f dashboard/Dockerfile -t edgeos-dashboard .
# (needs the whole module, not just dashboard/, since it imports edgeos/internal/...)

FROM golang:1.25-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/dashboard ./dashboard

FROM alpine:3.20
RUN apk add --no-cache ca-certificates && \
    adduser -D -u 10001 edgeos
USER edgeos
COPY --from=build /out/dashboard /usr/local/bin/dashboard
EXPOSE 8092
ENTRYPOINT ["/usr/local/bin/dashboard"]
