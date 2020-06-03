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
	Projects      *[]ProjectStats
	MergeRequests *[]MergeRequestStats
	Approvals     *[]ApprovalStats
}

//ProjectStats is the struct for Gitlab projects data we want.
type ProjectStats struct {
	ID                string
	PathWithNamespace string
}

//MergeRequestStats is the struct for Gitlab Merge Requests data we want
type MergeRequestStats struct {
	ID           string
	InternalID   int
	State        string
	LastUpdated  *time.Time
	TargetBranch string
	ProjectID    string
	ChangeCount  string
	Title        string
}

//ApprovalStats is the struct for Gitlab Approvals data we want
type ApprovalStats struct {
	Approvals int
	ID        string
	ProjectID string
}

//ExporterClient contains Gitlab information for connecting
type ExporterClient struct {
	gitlabURI    string
	gitlabAPIKey string
}

//New returns a new Client connection to Gitlab.
func New(c internal.Config) *ExporterClient {
	return &ExporterClient{
		gitlabAPIKey: c.GitlabAPIKey,
		gitlabURI:    c.GitlabURI,
	}
}

//GetStats retrieves data from API to create metrics from.
func (c *ExporterClient) GetStats() (*Stats, error) {

	httpClient := &http.Client{Timeout: 10 * time.Second}

	glc, err := gitlab.NewClient(c.gitlabAPIKey, gitlab.WithBaseURL(c.gitlabURI), gitlab.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	projects, err := getProjects(glc)
	if err != nil {
		return nil, err
	}

	mr, err := getMergeRequests(glc)
	if err != nil {
		return nil, err
	}

	appr, err := getApprovals(glc, *mr)

	return &Stats{
		Projects:      projects,
		MergeRequests: mr,
		Approvals: appr,
	}, nil
}

//getProjectStats retrieves all projects from Gitlab.
func getProjects(c *gitlab.Client) (*[]ProjectStats, error) {
	var result []ProjectStats
	var projectsTotal []*gitlab.Project

	page := 1

	for {
		projects, _, err := c.Projects.ListProjects(&gitlab.ListProjectsOptions{
			ListOptions: gitlab.ListOptions{Page: page, PerPage: 100},
			Archived:    gitlab.Bool(false),
			Simple:      gitlab.Bool(true),
		})
		if err != nil {
			return nil, err
		}

		if len(projects) == 0 {
			break
		}
		projectsTotal = append(projectsTotal, projects...)
		page++
	}

	log.Info("found a total of: ", len(projectsTotal), " projects")

	for _, project := range projectsTotal {
		result = append(result, ProjectStats{
			ID:                strconv.Itoa(project.ID),
			PathWithNamespace: project.PathWithNamespace,
		})
	}

	return &result, nil
}

//getMergeRequests retrieves all MRs targeted for the master branch for the last 7 days.
func getMergeRequests(c *gitlab.Client) (*[]MergeRequestStats, error) {
	var result []MergeRequestStats
	var mrTotal []*gitlab.MergeRequest

	updateAfter := time.Now().Add(-7 * 24 * time.Hour)

	page := 1

	for {
		mr, _, err := c.MergeRequests.ListMergeRequests(&gitlab.ListMergeRequestsOptions{
			ListOptions:  gitlab.ListOptions{Page: page, PerPage: 100},
			UpdatedAfter: &updateAfter,
			TargetBranch: gitlab.String("master"),
			Scope:        gitlab.String("all"),
			WIP:          gitlab.String("no"),
		})
		if err != nil {
			return nil, err
		}

		if len(mr) == 0 {
			break
		}

		mrTotal = append(mrTotal, mr...)
		page++
	}

	log.Info("Found a total of: ", len(mrTotal), " merge requests")

	for _, mr := range mrTotal {

		changeCount, _, err := c.MergeRequests.GetMergeRequest(mr.ProjectID, mr.IID, &gitlab.GetMergeRequestsOptions{})
		if err != nil {
			return nil, err
		}

		result = append(result, MergeRequestStats{
			LastUpdated:  mr.UpdatedAt,
			ProjectID:    strconv.Itoa(mr.ProjectID),
			State:        mr.State,
			TargetBranch: mr.TargetBranch,
			Title:        mr.Title,
			ID:           strconv.Itoa(mr.ID),
			InternalID:   mr.IID,
			ChangeCount:  changeCount.ChangesCount,
		})

	}

	return &result, nil
}

// getApprovals retrieves the amount of approvals left for a merge request
func getApprovals(c *gitlab.Client, mergeStats []MergeRequestStats) (*[]ApprovalStats, error) {
	var result []ApprovalStats

	for _, mr := range mergeStats {
		if mr.State == "opened" {
			approvals, _, err := c.MergeRequestApprovals.GetConfiguration(mr.ProjectID, mr.InternalID)
			if err != nil {
				return nil, err
			}

			result = append(result, ApprovalStats{
				Approvals: approvals.ApprovalsLeft,
				ID:        mr.ID,
				ProjectID: mr.ProjectID,
			})
		}
	}

	return &result, nil
}
