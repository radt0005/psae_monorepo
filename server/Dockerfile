# Multi-stage build for the spade-scheduler binary.
#
# Stage 1 compiles a static binary; stage 2 packages it into a minimal
# image suitable for DO App Platform (`hosting.md` §3, §6.4).
#
# The build context must be the *monorepo root* (`docker build -f
# server/Dockerfile .`) because go.mod uses `replace` directives that
# point at sibling modules `../core`, `../runner`, and `../captoken`.

FROM golang:1.25-bookworm AS build
WORKDIR /src

# Copy sibling modules so `replace` resolves at build time.
COPY core ./core
COPY runner ./runner
COPY captoken ./captoken
COPY server ./server

WORKDIR /src/server
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/spade-scheduler ./cmd/spade-scheduler

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/spade-scheduler /usr/local/bin/spade-scheduler
EXPOSE 1323
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/spade-scheduler"]
