# cStatsExporter
Windows Docker Stats exporter for Prometheus.io. Compatible with cadvisor metrics !

Cadvisor is doing a very good job on linux while on windows there isn't anything.

It is exposing a subset of the cadvisor metrics depending on what is available on a windows host:

## Run with docker
```
docker run --rm -p 9030:9030  -v \\.\pipe\docker_engine:\\.\pipe\docker_engine alexvaut/cstatsexporter:latest
```
## Run with docker-compose
```
services:
  cstatsexporter:
    image: alexvaut/cstatsexporter:latest
    volumes:
      - source: '\\.\pipe\docker_engine'
        target: '\\.\pipe\docker_engine'
        type: npipe    
```
## Deploy with docker-compose on a docker swarm cluster
```
services:
  cstatsexporter:
    image: alexvaut/cstatsexporter:latest
    volumes:
      - source: '\\.\pipe\docker_engine'
        target: '\\.\pipe\docker_engine'
        type: npipe
    deploy:
      mode: global
      placement:        
        constraints:
          - "node.platform.os == windows"              
```

## Build the docker image
```
go build -o main.exe .
docker build -t cstatsexporter .
```
## Configuration
In config.yaml:
```yaml
scrapeIntervalSeconds: 5
port: 9030
```
All the configuration parameters can be setup through environment variables. For instance, for port, setup the environment variable PORT.


## Metrics:
Accessible from http://localhost:9030/metrics when publishing port 9030.
```
# HELP container_cpu_system_seconds_total Cumulative system cpu time consumed in seconds.
# TYPE container_cpu_system_seconds_total counter
# HELP container_cpu_usage_seconds_total Cumulative cpu time consumed in seconds.
# TYPE container_cpu_usage_seconds_total counter
# HELP container_fs_reads_bytes_total Cumulative count of bytes read.
# TYPE container_fs_reads_bytes_total counter
# HELP container_fs_reads_total Cumulative count of reads completed.
# TYPE container_fs_reads_total counter
# HELP container_fs_writes_bytes_total Cumulative count of bytes written.
# TYPE container_fs_writes_bytes_total counter
# HELP container_fs_writes_total Cumulative count of writes completed.
# TYPE container_fs_writes_total counter
# HELP container_memory_usage_bytes Current memory usage in bytes, including all memory regardless of when it was accessed.
# TYPE container_memory_usage_bytes gauge
# HELP container_memory_working_set_bytes Current working set in bytes.
# TYPE container_memory_working_set_bytes gauge
# HELP container_network_receive_bytes_total Cumulative count of bytes received.
# TYPE container_network_receive_bytes_total counter
# HELP container_network_receive_errors_total Cumulative count of errors encountered while receiving.
# TYPE container_network_receive_errors_total counter
# HELP container_network_receive_packets_dropped_total Cumulative count of packets dropped while receiving.
# TYPE container_network_receive_packets_dropped_total counter
# HELP container_network_receive_packets_total Cumulative count of packets received.
# TYPE container_network_receive_packets_total counter
# HELP container_network_transmit_bytes_total Cumulative count of bytes transmitted.
# TYPE container_network_transmit_bytes_total counter
# HELP container_network_transmit_errors_total Cumulative count of errors encountered while transmitting.
# TYPE container_network_transmit_errors_total counter
# HELP container_network_transmit_packets_dropped_total Cumulative count of packets dropped while transmitting.
# TYPE container_network_transmit_packets_dropped_total counter
# HELP container_network_transmit_packets_total Cumulative count of packets transmitted.
# TYPE container_network_transmit_packets_total counter
# HELP container_spec_cpu_quota CPU quota of the container.
# TYPE container_spec_cpu_quota gauge
# HELP container_spec_memory_limit_bytes Memory limit for the container.
# TYPE container_spec_memory_limit_bytes gauge
# HELP node_meta Meta information on docker swarm nodes.
# TYPE node_meta gauge
# HELP host_meta Meta information on docker swarm container hosts.
# TYPE host_meta gauge
```

## Metadata:

- node_meta can be used as a lookup to link a **node id** with its **node name** since each container deployed as a service in a docker swarm cluster has a label with its node id. The 3 labels are
  - node_name
  - node_id
  - container_label_com_docker_swarm_node_id
  The idea is from https://github.com/stefanprodan/swarmprom    
- host_meta an be used as a lookup to link a **container hostname** with its **node name**. The 2 labels are
  - node_name
  - host
  It's helpful when used in conjonction with telegraf(https://github.com/influxdata/telegraf) which is setting up a host label on each metric.
