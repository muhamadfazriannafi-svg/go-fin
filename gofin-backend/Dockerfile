FROM golang:1.27-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o gofin .

FROM alpine:latest
RUN apk add --no-cache ca-certificates
WORKDIR /app
COPY --from=build /app/gofin .
CMD ["./gofin"]
