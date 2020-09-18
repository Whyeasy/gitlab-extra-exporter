//Package client contains all the files to extract the information from gitlab
package client

import (
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/whyeasy/gitlab-extra-exporter/internal"
	gitlab "github.com/xanzy/go-gitlab"
)

//Stats struct is the list of expected to results to export.
type Stats struct {
	Projects            *[]ProjectStats
	MergeRequests       *[]MergeRequestStats
	MergeRequestsOpen   *[]MergeRequestStats
	MergeRequestsClosed *[]MergeClosedStats
	MergeRequestsMerged *[]MergeMergedStats
	Approvals           *[]ApprovalStats
	Changes             *[]ChangeStats
}

//ExporterClient contains Gitlab information for connecting
type ExporterClient struct {
	gitlabURI    string
	gitlabAPIKey string
	httpClient   *http.Client
	interval     time.Duration
}

//New returns a new Client connection to Gitlab.
func New(c internal.Config) *ExporterClient {

	convertedTime, _ := strconv.ParseInt(c.Interval, 10, 64)

	exporter := &ExporterClient{
		gitlabAPIKey: c.GitlabAPIKey,
		gitlabURI:    c.GitlabURI,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		interval:     time.Duration(convertedTime),
	}

	exporter.startFetchData()

	return exporter
}

// CachedStats is to store scraped data for caching purposes.
var CachedStats *Stats = &Stats{
	Projects:            &[]ProjectStats{},
	MergeRequests:       &[]MergeRequestStats{},
	MergeRequestsOpen:   &[]MergeRequestStats{},
	MergeRequestsClosed: &[]MergeClosedStats{},
	MergeRequestsMerged: &[]MergeMergedStats{},
	Approvals:           &[]ApprovalStats{},
	Changes:             &[]ChangeStats{},
}

//GetStats retrieves data from API to create metrics from.
func (c *ExporterClient) GetStats() (*Stats, error) {

	return CachedStats, nil
}

func (c *ExporterClient) getData() error {

	glc, err := gitlab.NewClient(c.gitlabAPIKey, gitlab.WithBaseURL(c.gitlabURI), gitlab.WithHTTPClient(c.httpClient))
	if err != nil {
		return err
	}

	projects, err := getProjects(glc)
	if err != nil {
		return err
	}

	mrs, err := getMergeRequest(glc)
	if err != nil {
		return err
	}

	mrOpen, mrMerged, mrClosed, err := getMergeRequestsDetails(glc, *mrs)
	if err != nil {
		return err
	}

	approvals, err := getApprovals(glc, *mrOpen)
	if err != nil {
		return err
	}

	changes, err := getChanges(glc, *mrOpen)
	if err != nil {
		return err
	}

	CachedStats = &Stats{
		Projects:            projects,
		MergeRequests:       mrs,
		MergeRequestsOpen:   mrOpen,
		MergeRequestsClosed: mrClosed,
		MergeRequestsMerged: mrMerged,
		Approvals:           approvals,
		Changes:             changes,
	}

	log.Info("New data retrieved.")

	return nil
}

func (c *ExporterClient) startFetchData() {

	// Do initial call to have data from the start.
	go func() {
		err := c.getData()
		if err != nil {
			log.Error("Scraping failed.")
		}
	}()

	ticker := time.NewTicker(c.interval * time.Second)
	quit := make(chan struct{})

	go func() {
		for {
			select {
			case <-ticker.C:
				err := c.getData()
				if err != nil {
					log.Error("Scraping failed.")
				}
			case <-quit:
				ticker.Stop()
				return
			}
		}
	}()
}
