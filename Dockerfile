FROM golang:alpine

WORKDIR /app

COPY . .

RUN go install

ENTRYPOINT [ "nepbot" ]