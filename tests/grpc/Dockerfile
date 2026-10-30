FROM golang:1.27

WORKDIR /app

COPY pki ./pki
RUN chmod -R a+rX,go-w ./pki
COPY go.mod go.sum ./
COPY mutual-tls-server.go ./

RUN go build -o server ./mutual-tls-server.go

USER nobody

EXPOSE 50051

ENTRYPOINT [ "./server" ]
