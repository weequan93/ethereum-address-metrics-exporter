FROM debian:latest
WORKDIR /opt/ethereum-address-metrics-exporter
COPY ethereum-address-metrics-exporter* /ethereum-address-metrics-exporter
ENTRYPOINT ["/ethereum-address-metrics-exporter"]
