FROM golang:1.20-alpine AS build_base

RUN apk add --no-cache git

# Set the Current Working Directory inside the container
WORKDIR /tmp/app

# We want to populate the module cache based on the go.{mod,sum} files.
COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

# Unit tests
#RUN CGO_ENABLED=0 go test -v

# Build the Go app
RUN go mod download && CGO_ENABLED=0 go build -o ./dist/app cmd/app/main.go

# Start fresh from a smaller image
FROM alpine:3.18
RUN apk add ca-certificates
ENV SRV_ADDR=9090
COPY --from=build_base /tmp/app/dist/app /app/app

# This container exposes port 8080 to the outside world
EXPOSE $SRV_ADDR

# Run the binary program produced by `go install`
CMD ["/app/app"]
