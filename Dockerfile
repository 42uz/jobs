FROM golang:1.25 as builder

WORKDIR /app

COPY . .

RUN CGO_ENALBED=0 go build -o jobs42 cmd/server/main.go

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/jobs42 .

ENTRYPOINT [ "./jobs42" ]
