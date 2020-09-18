package internal

//Config struct for holding config for exporter and Gitlab
type Config struct {
	ListenAddress string
	ListenPath    string
	GitlabURI     string
	GitlabAPIKey  string
	Interval      string
}
