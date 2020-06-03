![build](https://github.com/Whyeasy/gitlab-extra-exporter/workflows/build/badge.svg)
![status-badge](https://goreportcard.com/badge/github.com/Whyeasy/gitlab-extra-exporter)
![Github go.mod Go version](https://img.shields.io/github/go-mod/go-version/Whyeasy/gitlab-extra-exporter)

# gitlab-extra-exporter

This is a Prometheus exporter for Gitlab to get information via the API.

Currently this exporter retrieves the following data:

- All projects within Gitlab
- Retrieves all Merge Request from the last 7 days with:
  - Last update done to the MR.
  - Amount of changes within the MR.

Because of the amount of API request done to get the amount of changes on a MR, limit this exporter to be only requested once per 5 minutes for example, with a Service Monitor time out of 30 sec (depending on the amount of MRs).

## Requirements

### Required

Provide your Gitlab URI; `--gitlabURI <string>` or as env variable `GITLAB_URI`.

Provide a Gitlab API Key with access to projects and merge requests; `--gitlabAPIKey <string>` or as env variables `GITLAB_API_KEY`

### Optional

Change listening port of the exporter; `listenAddress <string>` or as env variable `LISTEN_ADDRESS`. Default = `8080`

Change listening path of the exporter; `listenPath <string>` or as env variable `LISTEN_PATH`. Default = `/metrics`

## Helm

You can find a helm chart to install the exporter [here](https://github.com/Whyeasy/helm-charts/tree/master/charts/gitlab-extra-exporter).
