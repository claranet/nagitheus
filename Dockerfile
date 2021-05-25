FROM golang:1.15-alpine

WORKDIR /go/src/nagitheus
COPY . .

RUN go build 
RUN go install

CMD ["nagitheus"]
