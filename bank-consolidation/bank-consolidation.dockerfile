FROM golang:1.24-alpine


# Install dependencies
RUN apk add --no-cache tzdata git
ENV TZ=Asia/Jakarta

WORKDIR /app

# Install air untuk live reload
RUN go install github.com/air-verse/air@v1.62.0

COPY .env ./

# Copy hanya yang diperlukan untuk development
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Default command untuk development
CMD ["air"]
