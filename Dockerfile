# https://blog.codeship.com/building-minimal-docker-containers-for-go-applications/
FROM scratch
ADD build/wclip.docker /wclip.docker
ENV PORT=80
# The default bind is loopback-only, which would make the container
# unreachable from the host. Listen on all interfaces inside the
# container; expose/restrict at the docker -p / network layer.
ENV BIND=*
EXPOSE 80
CMD ["/wclip.docker"]
