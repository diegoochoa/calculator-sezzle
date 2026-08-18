# syntax=docker/dockerfile:1

# ---------------------------------------------------------------- build stage
FROM golang:1.25-alpine AS build

WORKDIR /src

# Dependencies are their own layer so source edits do not re-download them.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=docker
# CGO off and a static build, so the binary runs on a distroless base with no
# libc at all. -trimpath keeps build paths out of the binary.
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/server ./cmd/server

# --------------------------------------------------------------- test stage
# Built only when targeted: `docker build --target test .`
#
# Coverage is measured over ./internal/... to match the Makefile. Including
# cmd/ and scripts/ would drag the total down with packages that are wiring by
# design and deliberately untested.
FROM build AS test
RUN go vet ./... \
    && go test ./... \
    && go test ./internal/... -covermode=atomic -coverprofile=coverage.out \
    && ./scripts/coverage.sh coverage.out

# --------------------------------------------------------------- runtime stage
FROM gcr.io/distroless/static-debian12:nonroot AS runtime

# distroless/static carries no shell and no package manager, so there is
# nothing for an attacker who reaches RCE to pivot with.
COPY --from=build /out/server /server

USER nonroot:nonroot
EXPOSE 8080

ENV CALC_ENV=production \
    CALC_PORT=8080

ENTRYPOINT ["/server"]
