//Package client contains all the files to extract the information from gitlab
package client

import (
	"net/http"
	"time"

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
}

//New returns a new Client connection to Gitlab.
func New(c internal.Config) *ExporterClient {
	return &ExporterClient{
		gitlabAPIKey: c.GitlabAPIKey,
		gitlabURI:    c.GitlabURI,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
	}
}

//GetStats retrieves data from API to create metrics from.
func (c *ExporterClient) GetStats() (*Stats, error) {

	glc, err := gitlab.NewClient(c.gitlabAPIKey, gitlab.WithBaseURL(c.gitlabURI), gitlab.WithHTTPClient(c.httpClient))
	if err != nil {
		return nil, err
	}

	projects, err := getProjects(glc)
	if err != nil {
		return nil, err
	}

	mrs, err := getMergeRequest(glc)
	if err != nil {
		return nil, err
	}

	mrOpen, mrMerged, mrClosed, err := getMergeRequestsDetails(glc, *mrs)
	if err != nil {
		return nil, err
	}

	approvals, err := getApprovals(glc, *mrOpen)
	if err != nil {
		return nil, err
	}

	changes, err := getChanges(glc, *mrOpen)
	if err != nil {
		return nil, err
	}

	return &Stats{
		Projects:            projects,
		MergeRequests:       mrs,
		MergeRequestsOpen:   mrOpen,
		MergeRequestsClosed: mrClosed,
		MergeRequestsMerged: mrMerged,
		Approvals:           approvals,
		Changes:             changes,
	}, nil
}
