# ---- Build stage ----
FROM golang:1.24 AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /server .

# ---- Run stage ----
FROM gcr.io/distroless/static-debian12:nonroot
USER nonroot:nonroot

COPY --from=mwader/static-ffmpeg:7.0.2 /ffmpeg /usr/local/bin/ffmpeg
COPY --from=builder --chown=nonroot:nonroot /server /server

EXPOSE 8080

ENTRYPOINT ["/server"]
