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
	MergeRequests       *[]MergeRequestStats
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

	mrs, err := getMergeRequest(glc)
	if err != nil {
		return nil, err
	}

	mrOpen, mrMerged, mrClosed, err := getMergeRequestsDetails(glc, *mrs)
	if err != nil {
		return nil, err
	}

	appr, err := getApprovals(glc, *mrOpen)
	if err != nil {
		return nil, err
	}

	return &Stats{
		Projects:            projects,
		MergeRequests:       mrs,
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

//getMergeRequest retrieves all merge requests of the last 7 days
func getMergeRequest(c *gitlab.Client) (*[]MergeRequestStats, error) {

	updateAfter := time.Now().Add(-7 * 24 * time.Hour)
	var result []MergeRequestStats

	var mrTotal []*gitlab.MergeRequest

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

	log.Info("Found a total of: ", len(mrTotal), " MRs")

	for _, mr := range mrTotal {
		result = append(result, MergeRequestStats{
			ProjectID:    strconv.Itoa(mr.ProjectID),
			State:        mr.State,
			TargetBranch: mr.TargetBranch,
			Title:        mr.Title,
			ID:           strconv.Itoa(mr.ID),
			InternalID:   mr.IID,
		})
	}

	return &result, nil
}

//getMergeRequestsDetails retrieves the details of given MRs we need for metrics.
func getMergeRequestsDetails(c *gitlab.Client, mrs []MergeRequestStats) (*[]MergeRequestStats, *[]MergeMergedStats, *[]MergeClosedStats, error) {

	var mrOpen []MergeRequestStats
	var resultOpen *[]MergeRequestStats

	var mrMerged []MergeRequestStats
	var resultMerged *[]MergeMergedStats

	var mrClosed []MergeRequestStats
	var resultClosed *[]MergeClosedStats

	for _, mr := range mrs {
		switch {
		case mr.State == "opened":
			mrOpen = append(mrOpen, mr)
		case mr.State == "merged":
			mrMerged = append(mrMerged, mr)
		case mr.State == "closed":
			mrClosed = append(mrClosed, mr)
		}
	}

	var wg sync.WaitGroup

	errCh := make(chan error, 1)

	wg.Add(3)

	go func() {
		resultOpen = getOpenMergeRequests(c, errCh, &wg, mrOpen)
	}()

	go func() {
		resultMerged = getMergedMergeRequests(c, errCh, &wg, mrMerged)
	}()

	go func() {
		resultClosed = getClosedMergeRequests(c, errCh, &wg, mrClosed)
	}()

	wg.Wait()
	close(errCh)
	for err := range errCh {
		return nil, nil, nil, err
	}

	return resultOpen, resultMerged, resultClosed, nil
}

func getOpenMergeRequests(c *gitlab.Client, errCh chan<- error, wg *sync.WaitGroup, mergeStats []MergeRequestStats) *[]MergeRequestStats {

	var resultOpen []MergeRequestStats

	for _, mr := range mergeStats {

		result, _, err := c.MergeRequests.GetMergeRequest(mr.ProjectID, mr.InternalID, &gitlab.GetMergeRequestsOptions{})
		if err != nil {
			errCh <- err
			return nil
		}

		resultOpen = append(resultOpen, MergeRequestStats{
			ProjectID:   strconv.Itoa(result.ProjectID),
			ID:          strconv.Itoa(result.ID),
			InternalID:  result.IID,
			CreatedAt:   result.CreatedAt,
			LastUpdated: result.UpdatedAt,
			ChangeCount: result.ChangesCount,
			Assignees:   len(result.Assignees),
		})

	}
	log.Info(len(resultOpen), " Open MRs")
	wg.Done()

	return &resultOpen
}

func getMergedMergeRequests(c *gitlab.Client, errCh chan<- error, wg *sync.WaitGroup, mergeStats []MergeRequestStats) *[]MergeMergedStats {

	var resultMerged []MergeMergedStats

	for _, mr := range mergeStats {

		result, _, err := c.MergeRequests.GetMergeRequest(mr.ProjectID, mr.InternalID, &gitlab.GetMergeRequestsOptions{})
		if err != nil {
			errCh <- err
			return nil
		}

		if result.MergedAt == nil {
			result.MergedAt = &time.Time{}
		}

		resultMerged = append(resultMerged, MergeMergedStats{
			MergedAt: result.MergedAt,
			MergeRequest: MergeRequestStats{
				ProjectID:   strconv.Itoa(result.ProjectID),
				ID:          strconv.Itoa(result.ID),
				CreatedAt:   result.CreatedAt,
				LastUpdated: result.UpdatedAt,
				ChangeCount: result.ChangesCount,
				Assignees:   len(result.Assignees),
			},
		})
	}
	log.Info(len(resultMerged), " Merged MRs")
	wg.Done()

	return &resultMerged
}

func getClosedMergeRequests(c *gitlab.Client, errCh chan<- error, wg *sync.WaitGroup, mergeStats []MergeRequestStats) *[]MergeClosedStats {

	var resultClosed []MergeClosedStats

	for _, mr := range mergeStats {

		result, _, err := c.MergeRequests.GetMergeRequest(mr.ProjectID, mr.InternalID, &gitlab.GetMergeRequestsOptions{})
		if err != nil {
			errCh <- err
			return nil
		}

		resultClosed = append(resultClosed, MergeClosedStats{
			ClosedAt: result.ClosedAt,
			MergeRequest: MergeRequestStats{
				ProjectID:   strconv.Itoa(result.ProjectID),
				ID:          strconv.Itoa(result.ID),
				CreatedAt:   result.CreatedAt,
				LastUpdated: result.UpdatedAt,
				ChangeCount: result.ChangesCount,
				Assignees:   len(result.Assignees),
			},
		})

	}
	log.Info(len(resultClosed), " Closed MRs")
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
