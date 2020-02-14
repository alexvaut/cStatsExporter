package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync"
	"time"

	c "cstatsexporter/config"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	_ "github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/spf13/viper"
)

const HostnameLabel = "host"
const NodeNameLabel = "node_name"
const NodeIdLabel = "node_id"

var (
	nodeMeta                      *prometheus.GaugeVec
	hostMeta                      *prometheus.GaugeVec
	counterCpuUsageTotalSeconds   *prometheus.CounterVec
	counterCpuKernelTotalSeconds  *prometheus.CounterVec
	gaugeCpuLimitQuota            *prometheus.GaugeVec
	gaugeMemoryUsageBytes         *prometheus.GaugeVec
	gaugeMemoryWorkingSetBytes    *prometheus.GaugeVec
	gaugeMemoryLimitBytes         *prometheus.GaugeVec
	counterNetworkReceivedBytes   *prometheus.CounterVec
	counterNetworkReceivedErrors  *prometheus.CounterVec
	counterNetworkReceivedDropped *prometheus.CounterVec
	counterNetworkReceivedPackets *prometheus.CounterVec
	counterNetworkSentBytes       *prometheus.CounterVec
	counterNetworkSentErrors      *prometheus.CounterVec
	counterNetworkSentDropped     *prometheus.CounterVec
	counterNetworkSentPackets     *prometheus.CounterVec
	counterFsReadBytes            *prometheus.CounterVec
	counterFsReads                *prometheus.CounterVec
	counterFsWriteBytes           *prometheus.CounterVec
	counterFsWrites               *prometheus.CounterVec
	infos                         = map[string]types.ContainerJSON{}
	stats                         = map[string]types.StatsJSON{}
	nodeLabelNamesM               = prometheus.Labels{NodeIdLabel: "", "container_label_com_docker_swarm_node_id": "", NodeNameLabel: ""}
	hostLabelNamesM               = prometheus.Labels{HostnameLabel: "", NodeNameLabel: ""}
	labelNamesM                   = prometheus.Labels{}
	metrics                       = make([]interface{}, 0)
)

func WaitForCtrlC() {
	var end_waiter sync.WaitGroup
	end_waiter.Add(1)
	var signal_channel chan os.Signal
	signal_channel = make(chan os.Signal, 1)
	signal.Notify(signal_channel, os.Interrupt)
	go func() {
		<-signal_channel
		end_waiter.Done()
	}()
	end_waiter.Wait()
}

func startHttpServer(port int) *http.Server {
	srv := &http.Server{Addr: fmt.Sprintf(":%d", port)}

	go func() {
		// returns ErrServerClosed on graceful close
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			// NOTE: there is a chance that next line won't have time to run,
			// as main() doesn't wait for this goroutine to stop. don't use
			// code with race conditions like these for production. see post
			// comments below on more discussion on how to handle this.
			log.Fatalf("ListenAndServe(): %s", err)
		}
	}()

	// returning reference so caller can call Shutdown()
	return srv
}

func NormalizeLabel(label string) string {
	return "container_label_" + strings.ReplaceAll(label, ".", "_")
}

func GetKeys(m map[string]string) []string {
	ret := []string{}
	for k := range m {
		ret = append(ret, k)
	}
	return ret
}

func main() {

	fmt.Println("Starting...")
	var config c.Configurations = GetConfig()
	fmt.Printf("Configuration read. Scrap time = %d seconds.\n", config.ScrapeIntervalSeconds)

	GatherMetrics()

	//get the label names

	for _, info := range infos {
		//fmt.Println("Key:", key, "Value:", value)
		for labelName, _ := range info.Config.Labels {
			labelNamesM[NormalizeLabel(labelName)] = ""
		}
	}
	labelNamesM["id"] = ""
	labelNamesM["image"] = ""
	labelNamesM["name"] = ""

	labelNames := GetKeys(labelNamesM)

	netLabelNames := labelNames
	netLabelNames = append(netLabelNames, "interface")

	//create metrics
	nodeMeta = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "node_meta",
		Help: "Meta information on docker swarm nodes."}, GetKeys(nodeLabelNamesM))
	//no need to add in metrics

	hostMeta = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "host_meta",
		Help: "Meta information on docker swarm container hosts."}, GetKeys(hostLabelNamesM))
	//no need to add in metrics

	counterCpuUsageTotalSeconds = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_cpu_usage_seconds_total",
		Help: "Cumulative cpu time consumed in seconds."}, labelNames)
	metrics = append(metrics, counterCpuUsageTotalSeconds)

	counterCpuKernelTotalSeconds = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_cpu_system_seconds_total",
		Help: "Cumulative system cpu time consumed in seconds."}, labelNames)
	metrics = append(metrics, counterCpuKernelTotalSeconds)

	gaugeCpuLimitQuota = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "container_spec_cpu_quota",
		Help: "CPU quota of the container."}, labelNames)
	metrics = append(metrics, gaugeCpuLimitQuota)

	gaugeMemoryUsageBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "container_memory_usage_bytes",
		Help: "Current memory usage in bytes, including all memory regardless of when it was accessed."}, labelNames)
	metrics = append(metrics, gaugeMemoryUsageBytes)

	gaugeMemoryWorkingSetBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "container_memory_working_set_bytes",
		Help: "Current working set in bytes."}, labelNames)
	metrics = append(metrics, gaugeMemoryWorkingSetBytes)

	gaugeMemoryLimitBytes = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "container_spec_memory_limit_bytes",
		Help: "Memory limit for the container."}, labelNames)
	metrics = append(metrics, gaugeMemoryLimitBytes)

	counterNetworkReceivedBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_network_receive_bytes_total",
		Help: "Cumulative count of bytes received."}, netLabelNames)
	metrics = append(metrics, counterNetworkReceivedBytes)

	counterNetworkReceivedErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_network_receive_errors_total",
		Help: "Cumulative count of errors encountered while receiving."}, netLabelNames)
	metrics = append(metrics, counterNetworkReceivedErrors)

	counterNetworkReceivedDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_network_receive_packets_dropped_total",
		Help: "Cumulative count of packets dropped while receiving."}, netLabelNames)
	metrics = append(metrics, counterNetworkReceivedDropped)

	counterNetworkReceivedPackets = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_network_receive_packets_total",
		Help: "Cumulative count of packets received."}, netLabelNames)
	metrics = append(metrics, counterNetworkReceivedPackets)

	counterNetworkSentBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_network_transmit_bytes_total",
		Help: "Cumulative count of bytes transmitted."}, netLabelNames)
	metrics = append(metrics, counterNetworkSentBytes)

	counterNetworkSentErrors = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_network_transmit_errors_total",
		Help: "Cumulative count of errors encountered while transmitting."}, netLabelNames)
	metrics = append(metrics, counterNetworkSentErrors)

	counterNetworkSentDropped = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_network_transmit_packets_dropped_total",
		Help: "Cumulative count of packets dropped while transmitting."}, netLabelNames)
	metrics = append(metrics, counterNetworkSentDropped)

	counterNetworkSentPackets = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_network_transmit_packets_total",
		Help: "Cumulative count of packets transmitted."}, netLabelNames)
	metrics = append(metrics, counterNetworkSentPackets)

	counterFsReadBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_fs_reads_bytes_total",
		Help: "Cumulative count of bytes read."}, labelNames)
	metrics = append(metrics, counterFsReadBytes)

	counterFsReads = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_fs_reads_total",
		Help: "Cumulative count of reads completed."}, labelNames)
	metrics = append(metrics, counterFsReads)

	counterFsWriteBytes = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_fs_writes_bytes_total",
		Help: "Cumulative count of bytes written."}, labelNames)
	metrics = append(metrics, counterFsWriteBytes)

	counterFsWrites = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "container_fs_writes_total",
		Help: "Cumulative count of writes completed."}, labelNames)
	metrics = append(metrics, counterFsWrites)

	http.Handle("/metrics", promhttp.Handler())
	srv := startHttpServer(config.Port)
	fmt.Printf("Metrics server started on 'http://localhost:%d/metrics'\n", config.Port)

	go func() {
		for {
			GatherMetrics()
			time.Sleep(time.Duration(config.ScrapeIntervalSeconds) * time.Second)
		}
	}()

	WaitForCtrlC()
	fmt.Println("Exiting...")
	if err := srv.Shutdown(context.TODO()); err != nil {
		panic(err) // failure/timeout shutting down the server gracefully
	}

	fmt.Println("Exit.")
}

func GatherMetrics() {

	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(err)
	}

	var nodeData types.Info
	if nodeMeta != nil {
		nodeData, _ = cli.Info(context.Background())
		nodeMeta.WithLabelValues(nodeData.Swarm.NodeID, nodeData.Swarm.NodeID, nodeData.Name).Set(1)
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		panic(err)
	}

	for _, container := range containers {
		fmt.Printf("%s %s\n", container.ID[:12], container.Image)

		info, ok := infos[container.ID]
		if !ok {
			info, err = cli.ContainerInspect(context.Background(), container.ID)
			infos[container.ID] = info
			if err != nil {
				panic(err)
			}
		}

		cStats, err := cli.ContainerStats(context.Background(), container.ID, false)
		if err != nil {
			panic(err)
		}

		var labels = BuildLabels(container.ID)

		for labelName, labelValue := range info.Config.Labels {
			labels[NormalizeLabel(labelName)] = labelValue
		}

		for k, _ := range labelNamesM {
			if _, ok := labels[k]; !ok {
				labels[k] = ""
			}
		}

		var newStats types.StatsJSON

		decoder := json.NewDecoder(cStats.Body)
		for decoder.More() {
			decoder.Decode(&newStats)
		}

		if oldStats, ok := stats[container.ID]; ok {
			hostMeta.WithLabelValues(container.ID[0:12], nodeData.Name).Set(1)
			//CPU
			GetCounter(counterCpuUsageTotalSeconds, labels).Add(float64(newStats.CPUStats.CPUUsage.TotalUsage-oldStats.CPUStats.CPUUsage.TotalUsage) / 10000000)
			GetCounter(counterCpuKernelTotalSeconds, labels).Add(float64(newStats.CPUStats.CPUUsage.UsageInKernelmode-oldStats.CPUStats.CPUUsage.UsageInKernelmode) / 10000000)
			if info.HostConfig.NanoCPUs > 0 {
				GetGauge(gaugeCpuLimitQuota, labels).Set(float64(info.HostConfig.NanoCPUs) / 10000)
			}

			//Memory
			GetGauge(gaugeMemoryUsageBytes, labels).Set(float64(newStats.MemoryStats.Commit))
			GetGauge(gaugeMemoryWorkingSetBytes, labels).Set(float64(newStats.MemoryStats.PrivateWorkingSet))
			if info.HostConfig.Memory > 0 {
				GetGauge(gaugeMemoryLimitBytes, labels).Set(float64(info.HostConfig.Memory))
			}

			//IO
			GetCounter(counterFsReadBytes, labels).Add(float64(newStats.StorageStats.ReadSizeBytes - oldStats.StorageStats.ReadSizeBytes))
			GetCounter(counterFsReads, labels).Add(float64(newStats.StorageStats.ReadCountNormalized - oldStats.StorageStats.ReadCountNormalized))
			GetCounter(counterFsWriteBytes, labels).Add(float64(newStats.StorageStats.WriteSizeBytes - oldStats.StorageStats.WriteSizeBytes))
			GetCounter(counterFsWrites, labels).Add(float64(newStats.StorageStats.WriteCountNormalized - oldStats.StorageStats.WriteCountNormalized))

			for networkName, newNetworkStats := range newStats.Networks {
				if oldNetworkStats, ok := oldStats.Networks[networkName]; ok {
					labels["interface"] = networkName
					GetCounter(counterNetworkReceivedBytes, labels).Add(float64(newNetworkStats.RxBytes - oldNetworkStats.RxBytes))
					GetCounter(counterNetworkReceivedErrors, labels).Add(float64(newNetworkStats.RxErrors - oldNetworkStats.RxErrors))
					GetCounter(counterNetworkReceivedDropped, labels).Add(float64(newNetworkStats.RxDropped - oldNetworkStats.RxDropped))
					GetCounter(counterNetworkReceivedPackets, labels).Add(float64(newNetworkStats.RxPackets - oldNetworkStats.RxPackets))
					GetCounter(counterNetworkSentBytes, labels).Add(float64(newNetworkStats.TxBytes - oldNetworkStats.TxBytes))
					GetCounter(counterNetworkSentErrors, labels).Add(float64(newNetworkStats.TxErrors - oldNetworkStats.TxErrors))
					GetCounter(counterNetworkSentDropped, labels).Add(float64(newNetworkStats.TxDropped - oldNetworkStats.TxDropped))
					GetCounter(counterNetworkSentPackets, labels).Add(float64(newNetworkStats.TxPackets - oldNetworkStats.TxPackets))
				}
			}
		}

		stats[container.ID] = newStats
	}

	//for all existing container in the stats, check if the container still exists
	for id, stat := range stats {
		found := false
		for _, container := range containers {
			if container.ID == id {
				found = true
				break
			}
		}

		//if the container doesn't exist then delete all the related metrics
		if !found {
			fmt.Printf("Deleting metrics of %s\n", id[:10])

			var labels = BuildLabels(id) //all the metrics with the container id
			DeleteMetrics(labels)

			var labelsNet = BuildLabels(id) //all the metrics with container id and with the network interfaces on top
			for networkName, _ := range stat.Networks {
				labelsNet["interface"] = networkName
				DeleteMetrics(labelsNet)
			}

			hostMeta.Delete(prometheus.Labels{HostnameLabel: id[0:12], NodeNameLabel: nodeData.Name})

			delete(stats, id) //delete the container stats
			delete(infos, id) //delete the container info
		}
	}
}

func DeleteMetrics(labels prometheus.Labels) {
	for _, metric := range metrics {
		switch metric.(type) {
		case *prometheus.GaugeVec:
			metric.(*prometheus.GaugeVec).Delete(labels)
			break
		case *prometheus.CounterVec:
			metric.(*prometheus.CounterVec).Delete(labels)
			break
		}
	}
}

func BuildLabels(id string) prometheus.Labels {
	info := infos[id]
	var labels = prometheus.Labels{"id": "/docker/" + id, "image": info.Config.Image, "name": info.Name}
	return labels
}

func GetCounter(cVec *prometheus.CounterVec, labels prometheus.Labels) prometheus.Counter {
	counter, err := cVec.GetMetricWith(labels)
	if err != nil {
		panic(err)
	}
	return counter
}

func GetGauge(gVec *prometheus.GaugeVec, labels prometheus.Labels) prometheus.Gauge {
	gauge, err := gVec.GetMetricWith(labels)
	if err != nil {
		panic(err)
	}
	return gauge
}

func GetConfig() c.Configurations {
	// Set the file name of the configurations file
	viper.SetConfigName("config")

	// Set the path to look for the configurations file
	viper.AddConfigPath(".")

	// Enable VIPER to read Environment Variables
	viper.AutomaticEnv()
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.SetConfigType("yml")

	var configuration c.Configurations
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("Error reading config file, %s", err)
	}

	err := viper.Unmarshal(&configuration)
	if err != nil {
		fmt.Printf("Unable to decode into struct, %v", err)
	}

	return configuration
}
