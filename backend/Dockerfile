# --- build stage ---
FROM golang:1.25-alpine AS build
WORKDIR /src

# -mod=mod lets `go build` populate go.sum on first build (no committed go.sum yet).
ENV CGO_ENABLED=0 GOOS=linux GOFLAGS=-mod=mod

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/server ./cmd/server

# --- runtime stage ---
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /
COPY --from=build /out/server /server
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/server"]
