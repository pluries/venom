# Runs as root on purpose (SonarQube docker:S6471, accepted as safe by the maintainer, #2810): /workdir/results and /workdir/tests are
# meant to be bind-mounted, and a non-root user would fail to write to a host folder it does not own.
FROM alpine:3.24

RUN apk update && \
    apk --no-cache add tzdata ca-certificates && \
    rm -rf /var/cache/apk/*

COPY dist/venom.linux-amd64 /usr/local/bin/venom
RUN chmod +x /usr/local/bin/venom

VOLUME /workdir/results
VOLUME /workdir/tests
WORKDIR /workdir

ENV VENOM_OUTPUT_DIR=/workdir/results \
    VENOM_LIB_DIR=/workdir/tests/lib \
    VENOM_VERBOSE=1

ENTRYPOINT ["/usr/local/bin/venom"]
CMD ["run", "./tests/*.y*ml"]