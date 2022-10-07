# © 2022 Nokia.
#
# This code is a Contribution to the gNMIc project (“Work”) made under the Google Software Grant and Corporate Contributor License Agreement (“CLA”) and governed by the Apache License 2.0.
# No other rights or licenses in or to any of Nokia’s intellectual property are granted for any other purpose.
# This code is provided on an “as is” basis without any warranties of any kind.
#
# SPDX-License-Identifier: Apache-2.0

FROM scratch

LABEL maintainer="Karim Radhouani <medkarimrdi@gmail.com>, Roman Dodin <dodin.roman@gmail.com>"
LABEL documentation="https://gnmic.openconfig.net"
LABEL repo="https://github.com/openconfig/gnmic"

COPY gnmic /app/gnmic
ENTRYPOINT [ "/app/gnmic" ]
CMD [ "help" ]
