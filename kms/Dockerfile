# Multi-stage build for the spade-kms binary (spec/secrets.md §5).
#
# Stage 1 compiles a static binary; stage 2 packages it into a minimal
# image suitable for DO App Platform (hosting.md §2).
#
# The build context must be the *monorepo root* (`docker build -f
# kms/Dockerfile .`) because go.mod uses a `replace` directive pointing at
# the sibling module `../captoken`.

FROM golang:1.25-bookworm AS build
WORKDIR /src

# Copy sibling modules so `replace` resolves at build time.
COPY captoken ./captoken
COPY kms ./kms

WORKDIR /src/kms
RUN go mod download
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/spade-kms ./cmd/spade-kms

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/spade-kms /usr/local/bin/spade-kms
EXPOSE 8081
USER nonroot:nonroot
ENTRYPOINT ["/usr/local/bin/spade-kms"]
