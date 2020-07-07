package client

import (
	"strconv"

	log "github.com/sirupsen/logrus"
	gitlab "github.com/xanzy/go-gitlab"
)

//ProjectStats is the struct for Gitlab projects data we want.
type ProjectStats struct {
	ID                string
	PathWithNamespace string
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
