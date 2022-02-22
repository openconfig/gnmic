FROM alpine

LABEL maintainer="Karim Radhouani <medkarimrdi@gmail.com>, Roman Dodin <dodin.roman@gmail.com>"
LABEL documentation="https://gnmic.kmrd.dev"
LABEL repo="https://github.com/karimra/gnmic"

COPY gnmic /app/gnmic
ENTRYPOINT [ "/app/gnmic" ]
CMD [ "help" ]
