FROM golang:alpine AS build-env

RUN mkdir -p /go/src/app
WORKDIR /go/src/app

COPY ./src/*.go /go/src/app

ARG GOLDFLAGS="'-w -s'"
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

RUN apk --no-cache add --update git \
 && rm -rf /var/cache/apk/* \
 && go get -u github.com/golang/dep/cmd/dep \
 && dep init && dep ensure -vendor-only \
 && go build -a -ldflags "${GOLDFLAGS}" -installsuffix cgo -o goserver .

FROM alpine

#ADD ca-certificates.crt /etc/ssl/certs/
RUN apk --no-cache add --update ca-certificates \
 && rm -rf /var/cache/apk/*

COPY --from=build-env /go/src/app/goserver /app/

ARG PORT
ENV PORT ${PORT:-8081}

# Run as non root user
RUN addgroup -g 10001 -S app && \
    adduser -u 10001 -S app -G app 
USER app

EXPOSE ${PORT}

CMD ["/app/goserver"]
