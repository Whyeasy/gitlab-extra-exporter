package client

import (
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	gitlab "github.com/xanzy/go-gitlab"
)

//MergeClosedStats is the struct for closed merge requests
type MergeClosedStats struct {
	MergeRequest MergeRequestStats
	ClosedAt     *time.Time
	Duration     float64
}

//MergeMergedStats is the strucct for merged merge requests
type MergeMergedStats struct {
	MergeRequest MergeRequestStats
	MergedAt     *time.Time
	Duration     float64
}

//MergeRequestStats is the base struct for Gitlab Merge Requests data we want
type MergeRequestStats struct {
	ID           string
	InternalID   int
	State        string
	TargetBranch string
	SourceBranch string
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

type ChangeStats struct {
	ProjectID string
	ID        string
	Additions int
	Deletions int
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
			SourceBranch: mr.SourceBranch,
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
			ProjectID:    strconv.Itoa(result.ProjectID),
			ID:           strconv.Itoa(result.ID),
			InternalID:   result.IID,
			CreatedAt:    result.CreatedAt,
			LastUpdated:  result.UpdatedAt,
			ChangeCount:  result.ChangesCount,
			Assignees:    len(result.Assignees),
			SourceBranch: result.SourceBranch,
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

		if result.MergeError == "" {
			duration, _ := time.ParseDuration(result.MergedAt.Sub(*result.CreatedAt).String())

			resultMerged = append(resultMerged, MergeMergedStats{
				MergedAt: result.MergedAt,
				Duration: duration.Seconds(),
				MergeRequest: MergeRequestStats{
					ProjectID:    strconv.Itoa(result.ProjectID),
					ID:           strconv.Itoa(result.ID),
					CreatedAt:    result.CreatedAt,
					LastUpdated:  result.UpdatedAt,
					ChangeCount:  result.ChangesCount,
					Assignees:    len(result.Assignees),
					SourceBranch: result.SourceBranch,
				},
			})
		}
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

		if result.MergeError == "" {
			duration, _ := time.ParseDuration(result.ClosedAt.Sub(*result.CreatedAt).String())

			resultClosed = append(resultClosed, MergeClosedStats{
				ClosedAt: result.ClosedAt,
				Duration: duration.Seconds(),
				MergeRequest: MergeRequestStats{
					ProjectID:    strconv.Itoa(result.ProjectID),
					ID:           strconv.Itoa(result.ID),
					CreatedAt:    result.CreatedAt,
					LastUpdated:  result.UpdatedAt,
					ChangeCount:  result.ChangesCount,
					Assignees:    len(result.Assignees),
					SourceBranch: result.SourceBranch,
				},
			})
		}

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

func getChanges(c *gitlab.Client, mergeStats []MergeRequestStats) (*[]ChangeStats, error) {

	var result []ChangeStats

	for _, mr := range mergeStats {

		compareResult, _, err := c.Repositories.Compare(mr.ProjectID, &gitlab.CompareOptions{
			From: gitlab.String("master"),
			To:   gitlab.String(mr.SourceBranch),
		})
		if err != nil {
			return nil, err
		}

		additions := 0
		deletions := 0
		for _, diff := range compareResult.Diffs {
			additions += strings.Count(diff.Diff, "\n+")
			deletions += strings.Count(diff.Diff, "\n-")
		}

		result = append(result, ChangeStats{
			ID:        mr.ID,
			ProjectID: mr.ProjectID,
			Additions: additions,
			Deletions: deletions,
		})
	}

	return &result, nil
}
