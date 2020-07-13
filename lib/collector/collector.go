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

	projectInfo      *prometheus.Desc
	mergeRequestInfo *prometheus.Desc

	mergeRequestCreated      *prometheus.Desc
	mergeRequestMerged       *prometheus.Desc
	mergeRequestClosed       *prometheus.Desc
	mergeRequestUpdated      *prometheus.Desc
	mergeRequestChangedFiles *prometheus.Desc
	mergeRequestAssignees    *prometheus.Desc
	mergeRequestDuration     *prometheus.Desc

	//Details for Open Merge Requests
	mergeRequestApprovals *prometheus.Desc
	mergeRequestChanges   *prometheus.Desc
}

//New creates a new Collector with Prometheus descriptors.
func New(c *client.ExporterClient) *Collector {
	log.Info("Creating collector")
	return &Collector{
		up:     prometheus.NewDesc("gitlab_extra_up", "Whether Gitlab scrap was successful", nil, nil),
		client: c,

		projectInfo:      prometheus.NewDesc("gitlab_project_info", "General information about projects", []string{"project_id", "project_name"}, nil),
		mergeRequestInfo: prometheus.NewDesc("gitlab_merge_request_info", "General information about merge requests", []string{"merge_request_id", "target_branch", "state", "merge_request_title", "project_id", "merge_request_internal_id"}, nil),

		mergeRequestUpdated:      prometheus.NewDesc("gitlab_merge_request_updated", "Time since last update on the merge requests that are open", []string{"merge_request_id", "project_id"}, nil),
		mergeRequestClosed:       prometheus.NewDesc("gitlab_merge_request_closed", "Date of closing the merge request", []string{"merge_request_id", "project_id"}, nil),
		mergeRequestCreated:      prometheus.NewDesc("gitlab_merge_request_created", "Date of creating the merge request", []string{"merge_request_id", "project_id"}, nil),
		mergeRequestMerged:       prometheus.NewDesc("gitlab_merge_request_merged", "Date of merging the merge request", []string{"merge_request_id", "project_id"}, nil),
		mergeRequestChangedFiles: prometheus.NewDesc("gitlab_merge_request_changed_files", "Amount of changed files within the merge request", []string{"merge_request_id", "project_id"}, nil),
		mergeRequestAssignees:    prometheus.NewDesc("gitlab_merge_request_assignees", "Amount of assignees assigned to the MR", []string{"merge_request_id", "project_id"}, nil),
		mergeRequestDuration:     prometheus.NewDesc("gitlab_merge_request_duration", "Duration between creating and closing or merging a merge request", []string{"merge_request_id", "project_id"}, nil),

		//Details for Open Merge Requests
		mergeRequestApprovals: prometheus.NewDesc("gitlab_merge_request_approvals", "Amount of approvals left for approving MR", []string{"merge_request_id", "project_id"}, nil),
		mergeRequestChanges:   prometheus.NewDesc("gitlab_merge_request_changes", "Amount of additions and deletions within the merge request", []string{"merge_request_id", "project_id", "lines"}, nil),
	}
}

//Describe the metrics that are collected.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.up

	ch <- c.projectInfo
	ch <- c.mergeRequestInfo

	ch <- c.mergeRequestUpdated
	ch <- c.mergeRequestChangedFiles
	ch <- c.mergeRequestClosed
	ch <- c.mergeRequestCreated
	ch <- c.mergeRequestMerged
	ch <- c.mergeRequestAssignees
	ch <- c.mergeRequestDuration

	//Details for Open Merge Requests
	ch <- c.mergeRequestApprovals
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

		collectMergeReqeustInfo(c, ch, stats)

		collectOpenMergeRequestMetrics(c, ch, stats)

		collectClosedMergeRequestMetrics(c, ch, stats)

		collectMergedMergeRequestMetrics(c, ch, stats)

		collectMergeRequestApprovalMetrics(c, ch, stats)

		collectMergeRequestChanges(c, ch, stats)

		log.Info("Scrape Complete")
	}

}

func collectProjectInfo(c *Collector, ch chan<- prometheus.Metric, stats *client.Stats) {
	for _, project := range *stats.Projects {
		ch <- prometheus.MustNewConstMetric(c.projectInfo, prometheus.GaugeValue, 1, project.ID, project.PathWithNamespace)
	}
}

func collectMergeReqeustInfo(c *Collector, ch chan<- prometheus.Metric, stats *client.Stats) {
	for _, mr := range *stats.MergeRequests {
		ch <- prometheus.MustNewConstMetric(c.mergeRequestInfo, prometheus.GaugeValue, 1, mr.ID, mr.TargetBranch, mr.State, mr.Title, mr.ProjectID, strconv.Itoa(mr.InternalID))
	}
}

func collectOpenMergeRequestMetrics(c *Collector, ch chan<- prometheus.Metric, stats *client.Stats) {
	for _, mr := range *stats.MergeRequestsOpen {
		changes := 0.0
		if mr.ChangeCount == "1000+" {
			changes = 1000
		} else {
			changes, _ = strconv.ParseFloat(mr.ChangeCount, 64)
		}

		ch <- prometheus.MustNewConstMetric(c.mergeRequestCreated, prometheus.GaugeValue, float64(time.Time(*mr.CreatedAt).Unix()), mr.ID, mr.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestUpdated, prometheus.GaugeValue, time.Since(*mr.LastUpdated).Round(time.Second).Seconds(), mr.ID, mr.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestChangedFiles, prometheus.GaugeValue, changes, mr.ID, mr.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestAssignees, prometheus.GaugeValue, float64(mr.Assignees), mr.ID, mr.ProjectID)
	}
}

func collectClosedMergeRequestMetrics(c *Collector, ch chan<- prometheus.Metric, stats *client.Stats) {
	for _, mr := range *stats.MergeRequestsClosed {
		changes := 0.0
		if mr.MergeRequest.ChangeCount == "1000+" {
			changes = 1000
		} else {
			changes, _ = strconv.ParseFloat(mr.MergeRequest.ChangeCount, 64)
		}

		ch <- prometheus.MustNewConstMetric(c.mergeRequestCreated, prometheus.GaugeValue, float64(time.Time(*mr.MergeRequest.CreatedAt).Unix()), mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestUpdated, prometheus.GaugeValue, time.Since(*mr.MergeRequest.LastUpdated).Round(time.Second).Seconds(), mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestChangedFiles, prometheus.GaugeValue, changes, mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestClosed, prometheus.GaugeValue, float64(time.Time(*mr.ClosedAt).Unix()), mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestAssignees, prometheus.GaugeValue, float64(mr.MergeRequest.Assignees), mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestDuration, prometheus.GaugeValue, float64(mr.Duration), mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
	}
}

func collectMergedMergeRequestMetrics(c *Collector, ch chan<- prometheus.Metric, stats *client.Stats) {
	for _, mr := range *stats.MergeRequestsMerged {
		changes := 0.0
		if mr.MergeRequest.ChangeCount == "1000+" {
			changes = 1000
		} else {
			changes, _ = strconv.ParseFloat(mr.MergeRequest.ChangeCount, 64)
		}

		ch <- prometheus.MustNewConstMetric(c.mergeRequestCreated, prometheus.GaugeValue, float64(time.Time(*mr.MergeRequest.CreatedAt).Unix()), mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestUpdated, prometheus.GaugeValue, time.Since(*mr.MergeRequest.LastUpdated).Round(time.Second).Seconds(), mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestChangedFiles, prometheus.GaugeValue, changes, mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestMerged, prometheus.GaugeValue, float64(time.Time(*mr.MergedAt).Unix()), mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestAssignees, prometheus.GaugeValue, float64(mr.MergeRequest.Assignees), mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
		ch <- prometheus.MustNewConstMetric(c.mergeRequestDuration, prometheus.GaugeValue, float64(mr.Duration), mr.MergeRequest.ID, mr.MergeRequest.ProjectID)
	}
}

func collectMergeRequestApprovalMetrics(c *Collector, ch chan<- prometheus.Metric, stats *client.Stats) {
	for _, approval := range *stats.Approvals {
		ch <- prometheus.MustNewConstMetric(c.mergeRequestApprovals, prometheus.GaugeValue, float64(approval.Approvals), approval.ID, approval.ProjectID)
	}
}

func collectMergeRequestChanges(c *Collector, ch chan<- prometheus.Metric, stats *client.Stats) {
	for _, changes := range *stats.Changes {
		ch <- prometheus.MustNewConstMetric(c.mergeRequestChanges, prometheus.GaugeValue, float64(changes.Additions), changes.ID, changes.ProjectID, "added")
		ch <- prometheus.MustNewConstMetric(c.mergeRequestChanges, prometheus.GaugeValue, float64(changes.Deletions), changes.ID, changes.ProjectID, "deleted")
	}
}
