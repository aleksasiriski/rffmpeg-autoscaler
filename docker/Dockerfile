FROM sc4h/alpine-s6overlay:v2-3.15

ARG TARGETOS
ARG TARGETARCH
ARG TARGETVARIANT

ENV \
  PATH="/app/rffmpeg-autoscaler:${PATH}" \
  RFFMPEG_AUTOSCALER_CONFIG="/config" \
  RFFMPEG_AUTOSCALER_LOG="/config/log/rffmpeg-autoscaler.log" \
  RFFMPEG_AUTOSCALER_VERBOSITY="0"

# Binary
COPY ["docker/rffmpeg-autoscaler.yml.example", "/config/rffmpeg-autoscaler.yml"]

# Binary
COPY ["dist/rffmpeg-autoscaler_${TARGETOS}_${TARGETARCH}${TARGETVARIANT:+_7}/rffmpeg-autoscaler", "/app/rffmpeg-autoscaler/rffmpeg-autoscaler"]

# Add root files
COPY ["docker/run", "/etc/services.d/rffmpeg-autoscaler/run"]

# Volume
VOLUME ["/config"]