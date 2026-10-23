# Builds the Huddle API into a small image with no shell or compiler.
#   docker build -t huddle-api .

FROM golang:1.25.1-alpine AS build
WORKDIR /src

# Download modules first so code changes don't redo it.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/huddle .

# distroless/static has CA certificates (for Bachs, SMTP and Cloudinary over
# TLS) and runs as a non-root user.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/huddle /app/huddle
# Served at /doc/swagger.json for the Swagger UI.
COPY doc/swagger.json /app/doc/swagger.json

ENV PORT=8080
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/huddle"]
