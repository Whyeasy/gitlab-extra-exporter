FROM alpine

COPY gitlab-extra-exporter /usr/bin/
ENTRYPOINT ["/usr/bin/gitlab-extra-exporter"]