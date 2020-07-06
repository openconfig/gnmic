FROM golang:1.13 as builder
RUN mkdir /build 
ADD . /build/
WORKDIR /build 
RUN CGO_ENABLED=0 GOOS=linux go build -o gnmic .

FROM alpine
COPY --from=builder /build/gnmic /app/
COPY config.yaml ~/gnmic.yaml
WORKDIR /app
CMD ["./gnmic", "subscribe"]