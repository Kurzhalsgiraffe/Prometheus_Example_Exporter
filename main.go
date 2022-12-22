package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	yaml "gopkg.in/yaml.v2"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Structs for storing the config
type Config struct {
	Port int `yaml:"port"`
}
type ConfigWrapper struct {
	Conf Config
}

// Add new collectors here
type Collector struct {
	last_error                   error
	config                       *Config
	example_metric_with_label    *prometheus.Desc
	example_metric_without_label *prometheus.Desc
}

// Read config yaml file into Config struct
func readConf(filename string) (*Config, error) {
	buf, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	conf := &ConfigWrapper{}
	err = yaml.Unmarshal(buf, conf)
	if err != nil {
		return nil, fmt.Errorf("in file %q: %w", filename, err)
	}

	return &conf.Conf, err
}

// Dummy function to generate some data for the metrics
func getMetrics() ([]float64, error) {
	var list []float64
	for i := 0; i < 3; i++ {
		list = append(list, float64(i))
	}

	if len(list) == 0 {
		return nil, fmt.Errorf("error in getMetrics()")
	}

	return list, nil
}

// Create new Collectors with name, description and labels
func newCollector(config *Config) *Collector {
	return &Collector{
		last_error:                   nil,
		config:                       config,
		example_metric_with_label:    prometheus.NewDesc("example_metric_with_label", "Description", []string{"label_1", "label_2"}, nil),
		example_metric_without_label: prometheus.NewDesc("example_metric_without_label", "Description", nil, nil),
	}
}

func (collector *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.example_metric_with_label
	ch <- collector.example_metric_without_label
}

// This fuction runs when Prometheus scrapes the exporter.
func (collector *Collector) Collect(ch chan<- prometheus.Metric) {

	// Calling dummy function to get metrics
	list, err := getMetrics()
	if err != nil {
		if collector.last_error == nil || collector.last_error.Error() != err.Error() {
			log.Println("Failed to get Metrics:", err)
			collector.last_error = err
		}
	} else {
		collector.last_error = nil

		ch <- prometheus.MustNewConstMetric(collector.example_metric_without_label, prometheus.GaugeValue, float64(42))

		for i := 0; i < len(list); i++ {
			value := 42
			label_1 := fmt.Sprintf("%f", list[i])
			label_2 := "second label"
			ch <- prometheus.MustNewConstMetric(collector.example_metric_with_label, prometheus.GaugeValue, float64(value), label_1, label_2)
		}
	}
}

func serverMetrics(server *http.Server, metricsPath string) error {
	http.Handle(metricsPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`
            <html>
            <head><title>Example Exporter</title></head>
            <body>
            <h1>Example Exporter</h1>
            <p><a href='` + metricsPath + `'>Metrics</a></p>
            </body>
            </html>
        `))
	})
	return server.ListenAndServe()
}

func signalHandler(server *http.Server) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGKILL, syscall.SIGQUIT)

	// Block until a signal is received.
	s := <-c
	log.Println("Shutting down exporter, received signal:", s)
	err := server.Shutdown(context.Background())
	if err != nil {
		log.Println("Failed to shutdown server:", err)
	}
}

func main() {
	// Set or create logfile
	LOG_FILE := "./log/logfile"
	logFile, err := os.OpenFile(LOG_FILE, os.O_APPEND|os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		log.Fatal("Failed to create logfile: ", err)
	}

	defer logFile.Close()

	log.SetOutput(logFile)
	log.SetFlags(log.Lshortfile | log.LstdFlags)

	// Read Config
	fmt.Println(os.Args)
	if len(os.Args) < 2 {
		fmt.Println("Usage:", os.Args[0], "config-file")
		os.Exit(1)
	}
	c, err := readConf(os.Args[1])
	if err != nil {
		log.Fatal("Failed to read Config File: ", err)
	}

	collector := newCollector(c)
	prometheus.MustRegister(collector)

	server := &http.Server{Addr: fmt.Sprintf("%s%v", ":", c.Port), Handler: nil}
	log.Println("starting exporter on port", c.Port)

	go signalHandler(server)
	log.Println(serverMetrics(server, "/metrics"))
}
