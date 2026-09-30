# Self-host one playbook in a container.
#
#   docker build -t my-playbook .
#   docker run -p 4360:4360 -v playbook-data:/data my-playbook
#
# Inside the container the app always listens on 4360; pick the host side
# freely (-p 4362:4360). The playbook-setup-env skill keeps the TOOLCHAINS
# default below in sync with the playbook's toolchain scope.
#
# TOOLCHAINS picks what gets installed in the runtime image — install ONLY
# what this playbook's exercises use (see curriculum.md / the playbook-plan
# skill). Names come from content/toolchains.json; env/setup-debian.sh holds
# the install recipe for each (presets + whatever the playbook defines).
#
#   docker build --build-arg TOOLCHAINS="c python" -t my-playbook .

# --- build stage: pure-Go deps (modernc sqlite), so CGO can stay off ---
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /playbook .

# --- runtime stage ---
FROM debian:bookworm-slim
ARG TOOLCHAINS="c python"
# $ENV_DIR baked into the image (outside the /data volume so it isn't
# shadowed); rustup's rustc/cargo proxies read RUSTUP_HOME at run time
ENV PLAYBOOK_ENV_DIR=/opt/playbook-env RUSTUP_HOME=/opt/rustup
# the playground compiles and runs learner code — never run it as root;
# /data exists and belongs to that user, so a fresh volume inherits it
RUN useradd -m -u 10001 playbook && install -d -o playbook /data
COPY env/ /tmp/env/
# recipes run as root; afterwards the app user owns $ENV_DIR, because some
# tools write into their cache even offline (cargo's package lock, gradle)
RUN sh /tmp/env/setup-debian.sh $TOOLCHAINS && rm -rf /tmp/env \
 && chown -R playbook "$PLAYBOOK_ENV_DIR"
USER playbook
COPY --from=build /playbook /usr/local/bin/playbook
VOLUME /data
EXPOSE 4360
ENTRYPOINT ["playbook", "--no-open", "--host", "0.0.0.0", "--port", "4360", "--data", "/data"]
