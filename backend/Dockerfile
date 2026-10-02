# ForgeLab API (forgelab serve) for a container host such as Render.
# Build from the repository root, which holds learning/ and manifests/:
#   docker build -f backend/Dockerfile -t forgelab-api .
# Render gives PORT; FORGELAB_ALLOW_ORIGIN allows a dashboard on another
# origin. Saves and learning progress live under /app/.forgelab, which is
# lost when the container is replaced unless a disk is mounted there.

FROM golang:1.24-alpine AS build
WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/forgelab ./cmd/forgelab
# What the API reads at run time, and a writable place for what it saves.
COPY learning/ /out/app/learning/
COPY manifests/ /out/app/manifests/
RUN mkdir -p /out/app/.forgelab

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/forgelab /usr/local/bin/forgelab
COPY --from=build --chown=nonroot:nonroot /out/app /app
ENV PORT=8090
EXPOSE 8090
ENTRYPOINT ["/usr/local/bin/forgelab", "serve", "-repo", "/app"]
