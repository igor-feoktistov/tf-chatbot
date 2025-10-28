FROM golang:1.25.1-alpine

COPY . /usr/local/go/src/tf-chatbot

WORKDIR /usr/local/go/src/tf-chatbot

ENV GOLANG_VERSION=1.25.1

RUN go get -d -v ./...
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /usr/local/go/bin/tf-chatbot ./cmd/...

FROM alpine:3.22.1
COPY --from=0 /usr/local/go/bin/tf-chatbot /bin/tf-chatbot
CMD ["/bin/tf-chatbot"]
