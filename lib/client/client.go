//Package client contains all the files to extract the information from gitlab
package client

import (
	"net/http"
	"strconv"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/whyeasy/gitlab-extra-exporter/internal"
	gitlab "github.com/xanzy/go-gitlab"
)

//Stats struct is the list of expected to results to export.
type Stats struct {
	Projects            *[]ProjectStats
	MergeRequestsOpen   *[]MergeRequestStats
	MergeRequestsClosed *[]MergeClosedStats
	MergeRequestsMerged *[]MergeMergedStats
	Approvals           *[]ApprovalStats
}

//ProjectStats is the struct for Gitlab projects data we want.
type ProjectStats struct {
	ID                string
	PathWithNamespace string
}

//MergeClosedStats is the struct for closed merge requests
type MergeClosedStats struct {
	MergeRequest MergeRequestStats
	ClosedAt     *time.Time
}

//MergeMergedStats is the strucct for merged merge requests
type MergeMergedStats struct {
	MergeRequest MergeRequestStats
	MergedAt     *time.Time
}

//MergeRequestStats is the base struct for Gitlab Merge Requests data we want
type MergeRequestStats struct {
	ID           string
	InternalID   int
	State        string
	TargetBranch string
	ProjectID    string
	ChangeCount  string
	Title        string
	LastUpdated  *time.Time
	CreatedAt    *time.Time
	Assignees    int
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

	mrOpen, mrMerged, mrClosed, err := getMergeRequests(glc)
	if err != nil {
		return nil, err
	}

	appr, err := getApprovals(glc, *mrOpen)
	if err != nil {
		return nil, err
	}

	return &Stats{
		Projects:            projects,
		MergeRequestsOpen:   mrOpen,
		MergeRequestsClosed: mrClosed,
		MergeRequestsMerged: mrMerged,
		Approvals:           appr,
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
func getMergeRequests(c *gitlab.Client) (*[]MergeRequestStats, *[]MergeMergedStats, *[]MergeClosedStats, error) {

	var resultOpen *[]MergeRequestStats
	var resultMerged *[]MergeMergedStats
	var resultClosed *[]MergeClosedStats

	var wg sync.WaitGroup

	errCh := make(chan error, 1)

	wg.Add(3)

	go func() {
		resultOpen = getOpenMergeRequests(c, errCh, &wg)
	}()

	go func() {
		resultMerged = getMergedMergeRequests(c, errCh, &wg)
	}()

	go func() {
		resultClosed = getClosedMergeRequests(c, errCh, &wg)
	}()

	wg.Wait()
	close(errCh)
	for err := range errCh {
		return nil, nil, nil, err
	}

	return resultOpen, resultMerged, resultClosed, nil
}

func getOpenMergeRequests(c *gitlab.Client, errCh chan<- error, wg *sync.WaitGroup) *[]MergeRequestStats {

	var resultOpen []MergeRequestStats

	mrTotal, err := getMergeRequest(c, "opened")
	if err != nil {
		errCh <- err
		return nil
	}

	for _, mr := range mrTotal {

		changeCount, _, err := c.MergeRequests.GetMergeRequest(mr.ProjectID, mr.IID, &gitlab.GetMergeRequestsOptions{})
		if err != nil {
			errCh <- err
			return nil
		}

		resultOpen = append(resultOpen, MergeRequestStats{
			CreatedAt:    mr.CreatedAt,
			LastUpdated:  mr.UpdatedAt,
			ProjectID:    strconv.Itoa(mr.ProjectID),
			State:        mr.State,
			TargetBranch: mr.TargetBranch,
			Title:        mr.Title,
			ID:           strconv.Itoa(mr.ID),
			InternalID:   mr.IID,
			ChangeCount:  changeCount.ChangesCount,
			Assignees:    len(mr.Assignees),
		})

	}

	wg.Done()

	return &resultOpen
}

func getMergedMergeRequests(c *gitlab.Client, errCh chan<- error, wg *sync.WaitGroup) *[]MergeMergedStats {

	var resultMerged []MergeMergedStats

	mrTotal, err := getMergeRequest(c, "merged")
	if err != nil {
		errCh <- err
		return nil
	}

	for _, mr := range mrTotal {

		changeCount, _, err := c.MergeRequests.GetMergeRequest(mr.ProjectID, mr.IID, &gitlab.GetMergeRequestsOptions{})
		if err != nil {
			errCh <- err
			return nil
		}

		if mr.MergedAt == nil {
			mr.MergedAt = &time.Time{}
		}

		resultMerged = append(resultMerged, MergeMergedStats{
			MergedAt: mr.MergedAt,
			MergeRequest: MergeRequestStats{
				CreatedAt:    mr.CreatedAt,
				LastUpdated:  mr.UpdatedAt,
				ProjectID:    strconv.Itoa(mr.ProjectID),
				State:        mr.State,
				TargetBranch: mr.TargetBranch,
				Title:        mr.Title,
				ID:           strconv.Itoa(mr.ID),
				InternalID:   mr.IID,
				ChangeCount:  changeCount.ChangesCount,
				Assignees:    len(mr.Assignees),
			},
		})
	}

	wg.Done()

	return &resultMerged
}

func getClosedMergeRequests(c *gitlab.Client, errCh chan<- error, wg *sync.WaitGroup) *[]MergeClosedStats {

	var resultClosed []MergeClosedStats

	mrTotal, err := getMergeRequest(c, "closed")
	if err != nil {
		errCh <- err
		return nil
	}

	for _, mr := range mrTotal {

		changeCount, _, err := c.MergeRequests.GetMergeRequest(mr.ProjectID, mr.IID, &gitlab.GetMergeRequestsOptions{})
		if err != nil {
			errCh <- err
			return nil
		}

		resultClosed = append(resultClosed, MergeClosedStats{
			ClosedAt: mr.ClosedAt,
			MergeRequest: MergeRequestStats{
				CreatedAt:    mr.CreatedAt,
				LastUpdated:  mr.UpdatedAt,
				ProjectID:    strconv.Itoa(mr.ProjectID),
				State:        mr.State,
				TargetBranch: mr.TargetBranch,
				Title:        mr.Title,
				ID:           strconv.Itoa(mr.ID),
				InternalID:   mr.IID,
				ChangeCount:  changeCount.ChangesCount,
				Assignees:    len(mr.Assignees),
			},
		})

	}

	wg.Done()

	return &resultClosed
}

// getApprovals retrieves the amount of approvals left for a merge request
func getApprovals(c *gitlab.Client, mergeStats []MergeRequestStats) (*[]ApprovalStats, error) {
	var result []ApprovalStats

	for _, mr := range mergeStats {
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

	return &result, nil
}

func getMergeRequest(c *gitlab.Client, state string) ([]*gitlab.MergeRequest, error) {

	updateAfter := time.Now().Add(-7 * 24 * time.Hour)

	var mrTotal []*gitlab.MergeRequest

	page := 1

	for {
		mr, _, err := c.MergeRequests.ListMergeRequests(&gitlab.ListMergeRequestsOptions{
			ListOptions:  gitlab.ListOptions{Page: page, PerPage: 100},
			UpdatedAfter: &updateAfter,
			TargetBranch: gitlab.String("master"),
			Scope:        gitlab.String("all"),
			WIP:          gitlab.String("no"),
			State:        gitlab.String(state),
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

	log.Info("Found a total of: ", len(mrTotal), " merge requests with state: ", state)

	return mrTotal, nil
}
