# © 2022 Nokia.
#
# This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
# No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
# This code is provided on an “as is” basis without any warranties of any kind.
#
# SPDX-License-Identifier: Apache-2.0

FROM golang:1.24.12 AS builder

WORKDIR /build

COPY go.mod go.sum /build/
COPY pkg/api/go.mod pkg/api/go.sum /build/pkg/api/
COPY pkg/cache/go.mod pkg/cache/go.sum /build/pkg/cache/
RUN go mod download

ADD . /build

#RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o gnmic .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o gnmic .

FROM alpine
LABEL org.opencontainers.image.source=https://github.com/openconfig/gnmic
COPY --from=builder /build/gnmic /app/
WORKDIR /app
ENTRYPOINT [ "/app/gnmic" ]
CMD [ "help" ]
