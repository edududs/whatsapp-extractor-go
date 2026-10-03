FROM golang:1.27.1-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" -o /extractor ./cmd/whatsapp-extractor-go
RUN mkdir /data && chown 65532:65532 /data

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /extractor /whatsapp-extractor-go
COPY --from=build --chown=65532:65532 /data /data
USER 65532:65532
WORKDIR /
ENTRYPOINT ["/whatsapp-extractor-go"]
CMD ["run"]
