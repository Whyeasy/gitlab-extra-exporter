package main

import (
	"flag"
	"fmt"

	"net/http"
	"os"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"

	"github.com/whyeasy/gitlab-extra-exporter/internal"
	"github.com/whyeasy/gitlab-extra-exporter/lib/client"
	"github.com/whyeasy/gitlab-extra-exporter/lib/collector"
)

var (
	config internal.Config
)

func init() {
	flag.StringVar(&config.ListenAddress, "listenAddress", os.Getenv("LISTEN_ADDRESS"), "Port address of exporter to run on")
	flag.StringVar(&config.ListenPath, "listenPath", os.Getenv("LISTEN_PATH"), "Path where metrics will be exposed")
	flag.StringVar(&config.GitlabURI, "gitlabURI", os.Getenv("GITLAB_URI"), "URI to Gitlab instance to monitor")
	flag.StringVar(&config.GitlabAPIKey, "gitlabAPIKey", os.Getenv("GITLAB_API_KEY"), "API Key to access the Gitlab instance")
}

func main() {
	if err := parseConfig(); err != nil {
		log.Error(err)
		flag.Usage()
		os.Exit(2)
	}

	log.Info("Starting Gitlab Extra Exporter")

	client := client.New(config)
	coll := collector.New(client)
	prometheus.MustRegister(coll)

	log.Info("Start serving metrics")

	http.Handle(config.ListenPath, promhttp.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Gitlab Extra Exporter</title></head>
			<body>
			<h1>Gitlab Extra Exporter</h1>
			<p><a href="` + config.ListenPath + `">Metrics</a></p>
			</body>
			</html>`))
	})
	log.Fatal(http.ListenAndServe(":"+config.ListenAddress, nil))
}

func parseConfig() error {
	flag.Parse()
	required := []string{"gitlabURI", "gitlabAPIKey"}
	var err error
	flag.VisitAll(func(f *flag.Flag) {
		for _, r := range required {
			if r == f.Name && (f.Value.String() == "" || f.Value.String() == "0") {
				err = fmt.Errorf("%v is empty", f.Usage)
			}
		}
		if f.Name == "listenAddress" && (f.Value.String() == "" || f.Value.String() == "0") {
			err = f.Value.Set("8080")
			if err != nil {
				log.Error(err)
			}
		}
		if f.Name == "listenPath" && (f.Value.String() == "" || f.Value.String() == "0") {
			err = f.Value.Set("/metrics")
			if err != nil {
				log.Error(err)
			}
		}

	})
	return err
}
