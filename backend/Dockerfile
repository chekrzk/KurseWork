FROM golang:1.26-alpine AS build
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -o /api ./cmd/api
FROM alpine:3.22
RUN adduser -D pulse
USER pulse
COPY --from=build /api /api
ENTRYPOINT ["/api"]
