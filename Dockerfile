FROM mcr.microsoft.com/windows/nanoserver:1809-amd64

USER ContainerAdministrator

WORKDIR /app

# Expose port 9030 to the outside world
EXPOSE 9030

ARG DOCKER_API_VERSION=1.24

ENV DOCKER_API_VERSION=$DOCKER_API_VERSION

COPY config.yml /app/config.yml
COPY main.exe /app/

ENTRYPOINT ["C:/app/main.exe"]
