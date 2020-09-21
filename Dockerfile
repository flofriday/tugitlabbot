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
COPY --from=builder /app/gitlabbot ./
CMD ["./gitlabbot"]