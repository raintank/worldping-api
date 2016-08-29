FROM ubuntu:xenial
MAINTAINER Anthony Woods awoods@raintank.io

RUN apt-get update && apt-get -y install netcat-traditional ca-certificates iputils-ping

RUN mkdir -p /etc/raintank
COPY docker/worldping-api.ini /etc/raintank/worldping-api.ini

COPY build/worldping-api /usr/bin/worldping-api
COPY docker/entrypoint.sh /usr/bin/
RUN mkdir /usr/share/worldping-api
COPY build/public /usr/share/worldping-api/public
COPY build/conf /usr/share/worldping-api/conf

EXPOSE 80
EXPOSE 443

RUN mkdir /var/log/worldping-api
RUN mkdir /var/lib/worldping-api
VOLUME /var/log/worldping-api
VOLUME /var/lib/worldping-api

ENTRYPOINT ["/usr/bin/entrypoint.sh"]
CMD ["--config=/etc/raintank/worldping-api.ini", "--homepath=/usr/share/worldping-api/", "cfg:default.paths.data=/var/lib/worldping-api", "cfg:default.paths.logs=/var/log/worldping-api"]
