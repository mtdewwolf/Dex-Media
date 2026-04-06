FROM golang:1.22-alpine

# Install build tools for sqlite3 and ffmpeg
RUN apk add --no-cache gcc musl-dev ffmpeg sqlite

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod tidy && go mod download

COPY . .

# Enable CGO for sqlite3
ENV CGO_ENABLED=1

RUN go build -o main .

EXPOSE 8080

CMD ["./main"]
