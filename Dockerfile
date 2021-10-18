FROM golang:1.17.2 as builder
ADD . /build
WORKDIR /build
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o gnmic .

FROM alpine
LABEL org.opencontainers.image.source https://github.com/karimra/gnmic
COPY --from=builder /build/gnmic /app/
WORKDIR /app
ENTRYPOINT [ "/app/gnmic" ]
CMD [ "help" ]
