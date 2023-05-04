FROM gcr.io/distroless/static-debian11:nonroot

ENV RFFMPEG_AUTOSCALER_CONFIG="/home/nonroot" \
    RFFMPEG_AUTOSCALER_LOG="/home/nonroot/rffmpeg-autoscaler.log"

COPY rffmpeg-autoscaler .
COPY apps/rffmpeg-autoscaler/rffmpeg-autoscaler.yml .

ENTRYPOINT ["./rffmpeg-autoscaler"]

LABEL org.opencontainers.image.source="https://github.com/aleksasiriski/smerac-go"