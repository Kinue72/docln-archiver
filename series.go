package main

type Series struct {
	Id string `json:"id"`

	Title       string   `json:"title"`
	Cover       string   `json:"cover"`
	Description []string `json:"description"`

	Status string `json:"status"`

	Author string `json:"author"`
	Artist string `json:"artist"`

	Translator string `json:"translator"`
	Group      string `json:"group"`

	Volumes []Volume `json:"volumes"`

	BaseUrl string `json:"-"`
}

type Volume struct {
	Title    string    `json:"title"`
	Cover    string    `json:"cover"`
	Chapters []Chapter `json:"chapters"`
}

type Chapter struct {
	Title string `json:"title"`
	Url   string `json:"url"`
}
