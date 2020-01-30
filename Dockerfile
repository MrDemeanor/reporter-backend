FROM golang:1.13.6-alpine3.11

COPY . /go/src

RUN apk add --no-cache git

RUN go get github.com/google/uuid \
    && go get github.com/gorilla/handlers \
    && go get github.com/gorilla/mux \
    && go get github.com/tealeg/xlsx

EXPOSE 8080

WORKDIR /go/src

ENTRYPOINT ["go", "run", "main.go"]