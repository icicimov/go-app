FROM golang:alpine AS build-env

WORKDIR /go/src/app

COPY go.mod go.sum ./
RUN go mod download

COPY *.go favicon.ico ./

ENV CGO_ENABLED=0
ENV GOOS=linux

ARG GIT_TAG="v0.0.0"
ARG GIT_COMMIT="unknown"
ARG BUILD_TIME="unknown"

RUN go build \
    -ldflags "-w -s -X main.Release=${GIT_TAG} -X main.GitCommit=${GIT_COMMIT} -X main.BuildTime=${BUILD_TIME}" \
    -o goserver .

FROM alpine

RUN apk --no-cache add --update ca-certificates \
 && rm -rf /var/cache/apk/*

COPY --from=build-env /go/src/app/goserver /app/

# Run as non root user
RUN addgroup -g 10001 -S app && \
    adduser -u 10001 -S app -G app
USER app

EXPOSE 8081

CMD ["/app/goserver"]
