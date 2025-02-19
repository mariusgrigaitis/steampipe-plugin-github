package models

type Release struct {
	basicIdentifiers
	Author        BasicUser    `json:"author"`
	CreatedAt     NullableTime `json:"created_at"`
	Description   string       `json:"description"`
	IsDraft       bool         `json:"is_draft"`
	IsLatest      bool         `json:"is_latest"`
	IsPrerelease  bool         `json:"is_prerelease"`
	PublishedAt   NullableTime `json:"published_at"`
	Tag           BasicRef     `json:"tag"`
	TagCommit     BasicCommit  `json:"tag_commit"`
	TagName       string       `json:"tag_name"`
	UpdatedAt     NullableTime `json:"updated_at"`
	Url           string       `json:"url"`
	ReleaseAssets struct {
		Nodes []ReleaseAsset
	} `graphql:"releaseAssets(first: 100)" json:"assets"`
	CanReact bool `graphql:"canReact: viewerCanReact" json:"can_react"`
	// Mentions [pageable]
	// Reactions [pageable]
}

type ReleaseAsset struct {
	Id          string `json:"node_id"`
	ContentType string `json:"content_type"`
	Name        string `json:"name"`
	Size        int    `json:"size"`
}
