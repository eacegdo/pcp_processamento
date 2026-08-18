# Build
FROM golang:1.26-alpine AS build
WORKDIR /src

RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/pcp-processamento ./cmd/pcp-processamento

# Runtime
FROM alpine:3.22
RUN apk add --no-cache ca-certificates tzdata \
  && adduser -D -H -u 10001 app

WORKDIR /app
COPY --from=build /out/pcp-processamento /app/pcp-processamento

USER app
EXPOSE 8080

# Segredos só via env do EasyPanel (SUPABASE_*, API_KEY, …)
ENV HTTP_ADDR=:8080

CMD ["/app/pcp-processamento"]
