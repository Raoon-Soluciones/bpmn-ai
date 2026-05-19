FROM golang:1.26-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bpmn-ai ./cmd/engine

FROM alpine:3.20

RUN apk --no-cache add ca-certificates tzdata

RUN addgroup -S bpmn && adduser -S bpmn -G bpmn

USER bpmn

WORKDIR /app

COPY --from=builder /bpmn-ai /app/bpmn-ai

EXPOSE 8080

ENTRYPOINT ["/app/bpmn-ai"]
