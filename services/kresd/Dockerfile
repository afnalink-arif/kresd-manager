FROM cznic/knot-resolver:latest

USER root
RUN mkdir -p /var/cache/knot-resolver /run/knot-resolver /etc/knot-resolver/tls && \
    chown -R knot-resolver:knot-resolver /var/cache/knot-resolver /run/knot-resolver

USER knot-resolver
