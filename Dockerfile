# Use the alpine image to build
FROM golang:1.15-alpine AS builder
WORKDIR /app

# Install the dependencies 
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest and build the container
COPY . .
RUN CGO_ENABLE=0 go build -o gitlabbot

# Define the start point of the container
FROM alpine
# Install tzdata for the correct timezone in the final image
RUN apk add tzdata

# Set the timezone to europe
RUN cp /usr/share/zoneinfo/Europe/Vienna /etc/localtime && \
    echo "Europe/Vienna" >  /etc/timezone

# Copy the binary from the build container
WORKDIR /app
COPY --from=builder /app/gitlabbot ./
CMD ["./gitlabbot"]