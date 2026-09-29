# Self-host one playbook in a container.
#
#   docker build -t my-playbook .
#   docker run -p 4360:4360 -v playbook-data:/data my-playbook
#
# TOOLCHAINS picks what gets installed in the runtime image — install ONLY
# what this playbook's exercises use (see content/curriculum.md / the
# playbook-plan skill). Space-separated from: c cpp python node kotlin java
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
ARG TOOLCHAINS="c"
RUN set -eux; \
    apt-get update; \
    for t in $TOOLCHAINS; do \
      case "$t" in \
        c)      apt-get install -y --no-install-recommends gcc libc6-dev ;; \
        cpp)    apt-get install -y --no-install-recommends g++ libc6-dev ;; \
        python) apt-get install -y --no-install-recommends python3 ;; \
        node)   apt-get install -y --no-install-recommends nodejs ;; \
        java)   apt-get install -y --no-install-recommends default-jdk-headless ;; \
        kotlin) apt-get install -y --no-install-recommends default-jdk-headless unzip curl ca-certificates; \
                curl -fsSL -o /tmp/kotlin.zip https://github.com/JetBrains/kotlin/releases/download/v2.1.0/kotlin-compiler-2.1.0.zip; \
                unzip -q /tmp/kotlin.zip -d /opt; rm /tmp/kotlin.zip; \
                ln -s /opt/kotlinc/bin/kotlinc /opt/kotlinc/bin/kotlin /usr/local/bin/ ;; \
      esac; \
    done; \
    rm -rf /var/lib/apt/lists/*
# the playground compiles and runs learner code — never run it as root
RUN useradd -m -u 10001 playbook
USER playbook
COPY --from=build /playbook /usr/local/bin/playbook
VOLUME /data
EXPOSE 4360
ENTRYPOINT ["playbook", "--no-open", "--host", "0.0.0.0", "--data", "/data"]
