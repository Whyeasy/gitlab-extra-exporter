//Package collector contains all the go files needed to export metrics.
package collector

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"

	client "github.com/whyeasy/gitlab-extra-exporter/lib/client"
)

//Collector struct for holding Prometheus Desc and Exporter Client
type Collector struct {
	up     *prometheus.Desc
	client *client.ExporterClient

	projectInfo *prometheus.Desc

	mergeRequestUpdated *prometheus.Desc
	mergeRequestChanges *prometheus.Desc
}

//New creates a new Collector with Prometheus descriptors.
func New(c *client.ExporterClient) *Collector {
	log.Info("Creating collector")
	return &Collector{
		up:     prometheus.NewDesc("gitlab_extra_up", "Whether Gitlab scrap was successful", nil, nil),
		client: c,

		projectInfo:         prometheus.NewDesc("gitlab_project_info", "General information about projects", []string{"project_id", "project_name"}, nil),
		mergeRequestUpdated: prometheus.NewDesc("gitlab_merge_request_updated", "Last update on the merge requests that are open", []string{"merge_request_id", "target_branch", "state", "project_id"}, nil),
		mergeRequestChanges: prometheus.NewDesc("gitlab_merge_request_changes", "Amount of changed files within the merge request", []string{"merge_request_id", "target_branch", "state", "project_id"}, nil),
	}
}

//Describe the metrics that are collected.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up

	ch <- c.projectInfo
	ch <- c.mergeRequestUpdated
	ch <- c.mergeRequestChanges
}

//Collect gathers the metrics that are exported.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {

	log.Info("Running scrape")

	if stats, err := c.client.GetStats(); err != nil {
		log.Error(err)
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 0)
	} else {
		ch <- prometheus.MustNewConstMetric(c.up, prometheus.GaugeValue, 1)

		collectProjectInfo(c, ch, stats)

		collectMergeRequestMetrics(c, ch, stats)

		log.Info("Scrape Complete")
	}

}

//Gathering Project metrics

func collectProjectInfo(c *Collector, ch chan<- prometheus.Metric, stats *client.Stats) {
	for _, project := range *stats.Projects {
		ch <- prometheus.MustNewConstMetric(c.projectInfo, prometheus.GaugeValue, 1, project.ID, project.PathWithNamespace)
	}
}

func collectMergeRequestMetrics(c *Collector, ch chan<- prometheus.Metric, stats *client.Stats) {
	for _, mr := range *stats.MergeRequests {
		changes := 0.0
		if mr.ChangeCount == "1000+" {
			changes = 1000
		} else {
			changes, _ = strconv.ParseFloat(mr.ChangeCount, 64)
		}

		ch <- prometheus.MustNewConstMetric(c.mergeRequestUpdated, prometheus.GaugeValue, time.Since(*mr.LastUpdated).Round(time.Second).Seconds(), mr.ID, mr.TargetBranch, mr.State, mr.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestChanges, prometheus.GaugeValue, changes, mr.ID, mr.TargetBranch, mr.State, mr.ProjectID)
	}
}
