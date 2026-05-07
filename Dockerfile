# https://blog.codeship.com/building-minimal-docker-containers-for-go-applications/
FROM scratch
ADD build/wclip.docker /wclip.docker
ENV PORT 80
EXPOSE 80
CMD ["/wclip.docker"]
