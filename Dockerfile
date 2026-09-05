FROM golang:1.25

WORKDIR /app

COPY . .

RUN go build -o jobs42 cmd/server/main.go

ENTRYPOINT [ "./jobs42" ]
