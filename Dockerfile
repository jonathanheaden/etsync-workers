FROM golang:latest as builder

WORKDIR /go/src/etsync-code
COPY ./cmd/ /go/src/etsync-code
#RUN go install .
RUN go get -d -v ./... \
  && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o etsync .

FROM  google/cloud-sdk:206.0.0-alpine

WORKDIR /app
ENV PATH /app:$PATH

EXPOSE 8080
ENTRYPOINT ["/app/etsync"]

COPY --from=builder /go/src/etsync-code/ ./
