package github

import (
	"context"

	"github.com/shurcooL/githubv4"
	"github.com/turbot/steampipe-plugin-github/github/models"
	"github.com/turbot/steampipe-plugin-sdk/v5/grpc/proto"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin"
	"github.com/turbot/steampipe-plugin-sdk/v5/plugin/transform"
)

func tableGitHubRelease() *plugin.Table {
	return &plugin.Table{
		Name:        "github_release",
		Description: "GitHub Releases bundle project files for download by users.",
		List: &plugin.ListConfig{
			KeyColumns:        plugin.SingleColumn("repository_full_name"),
			ShouldIgnoreError: isNotFoundError([]string{"404"}),
			Hydrate:           tableGitHubReleaseList,
		},
		Get: &plugin.GetConfig{
			KeyColumns:        plugin.AllColumns([]string{"repository_full_name", "tag_name"}),
			ShouldIgnoreError: isNotFoundError([]string{"404"}),
			Hydrate:           tableGitHubReleaseGet,
		},
		Columns: commonColumns([]*plugin.Column{

			// Top columns
			{Name: "repository_full_name", Type: proto.ColumnType_STRING, Transform: transform.FromQual("repository_full_name"), Description: "Full name of the repository that contains the release."},
			{Name: "tag_name", Type: proto.ColumnType_STRING, Description: "The name of the tag the release is associated with."},

			// Other columns
			{Name: "author_login", Type: proto.ColumnType_STRING, Transform: transform.FromField("Author.Login"), Description: "The login name of the user that created the release."},
			{Name: "body", Type: proto.ColumnType_STRING, Transform: transform.FromField("Description"), Description: "Text describing the contents of the tag."},
			{Name: "created_at", Type: proto.ColumnType_TIMESTAMP, Transform: transform.FromField("CreatedAt").Transform(convertTimestamp), Description: "Time when the release was created."},
			{Name: "draft", Type: proto.ColumnType_BOOL, Transform: transform.FromField("IsDraft"), Description: "True if this is a draft (unpublished) release."},
			{Name: "html_url", Type: proto.ColumnType_STRING, Transform: transform.FromField("Url"), Description: "HTML URL for the release."},
			{Name: "id", Type: proto.ColumnType_INT, Transform: transform.FromField("Id"), Description: "Unique ID of the release."},
			{Name: "name", Type: proto.ColumnType_STRING, Description: "The name of the release."},
			{Name: "node_id", Type: proto.ColumnType_STRING, Transform: transform.FromField("NodeId"), Description: "Node where GitHub stores this data internally."},
			{Name: "prerelease", Type: proto.ColumnType_BOOL, Transform: transform.FromField("IsPrerelease"), Description: "True if this is a prerelease version."},
			{Name: "published_at", Type: proto.ColumnType_TIMESTAMP, Transform: transform.FromField("PublishedAt").NullIfZero().Transform(convertTimestamp), Description: "Time when the release was published."},
			// TODO: slightly different structure
			{Name: "assets", Type: proto.ColumnType_JSON, Transform: transform.FromField("ReleaseAssets.Nodes"), Description: "List of assets contained in the release."},

			// TODO: all these seem to contain url like https://api.github.com/...
			{Name: "url", Type: proto.ColumnType_STRING, Transform: transform.FromField("ResourcePath"), Description: "URL of the release."},
			{Name: "zipball_url", Type: proto.ColumnType_STRING, Transform: transform.FromField("ZipballUrl"), Description: "Zipball URL for the release."},
			{Name: "upload_url", Type: proto.ColumnType_STRING, Transform: transform.FromField("UploadUrl"), Description: "Upload URL for the release."},
			{Name: "tarball_url", Type: proto.ColumnType_STRING, Transform: transform.FromField("TarballUrl"), Description: "Tarball URL for the release."},
			{Name: "assets_url", Type: proto.ColumnType_STRING, Description: "Assets URL for the release."},

			// TODO: i couldn't find such field on GraphQL API
			{Name: "target_commitish", Type: proto.ColumnType_STRING, Transform: transform.FromField("Tag.Name"), Description: "Specifies the commitish value that determines where the Git tag is created from. Can be any branch or commit SHA."},
		}),
	}
}

func tableGitHubReleaseList(ctx context.Context, d *plugin.QueryData, h *plugin.HydrateData) (interface{}, error) {
	quals := d.EqualsQuals
	fullName := quals["repository_full_name"].GetStringValue()
	owner, repoName := parseRepoFullName(fullName)

	pageSize := adjustPageSize(100, d.QueryContext.Limit)

	var query struct {
		RateLimit  models.RateLimit
		Repository struct {
			Releases struct {
				PageInfo   models.PageInfo
				TotalCount int
				Nodes      []models.Release
			} `graphql:"releases(first: $pageSize, after: $cursor, orderBy: {field: CREATED_AT, direction: DESC})"`
		} `graphql:"repository(owner: $owner, name: $name)"`
	}

	variables := map[string]interface{}{
		"owner":    githubv4.String(owner),
		"name":     githubv4.String(repoName),
		"pageSize": githubv4.Int(pageSize),
		"cursor":   (*githubv4.String)(nil),
	}

	client := connectV4(ctx, d)

	for {
		err := client.Query(ctx, &query, variables)
		plugin.Logger(ctx).Debug(rateLimitLogString("github_release", &query.RateLimit))
		if err != nil {
			plugin.Logger(ctx).Error("github_release", "api_error", err)
			return nil, err
		}

		for _, release := range query.Repository.Releases.Nodes {
			d.StreamListItem(ctx, release)

			// Context can be cancelled due to manual cancellation or the limit has been hit
			if d.RowsRemaining(ctx) == 0 {
				return nil, nil
			}
		}

		if !query.Repository.Releases.PageInfo.HasNextPage {
			break
		}
		variables["cursor"] = githubv4.NewString(query.Repository.Releases.PageInfo.EndCursor)
	}

	return nil, nil
}

func tableGitHubReleaseGet(ctx context.Context, d *plugin.QueryData, _ *plugin.HydrateData) (interface{}, error) {
	quals := d.EqualsQuals
	tagName := quals["tag_name"].GetStringValue()
	fullName := quals["repository_full_name"].GetStringValue()
	owner, repo := parseRepoFullName(fullName)

	client := connectV4(ctx, d)

	var query struct {
		RateLimit  models.RateLimit
		Repository struct {
			Release models.Release `graphql:"release(tagName: $tagName)"`
		} `graphql:"repository(owner: $owner, name: $repo)"`
	}

	variables := map[string]interface{}{
		"owner":   githubv4.String(owner),
		"repo":    githubv4.String(repo),
		"tagName": githubv4.String(tagName),
	}

	err := client.Query(ctx, &query, variables)
	plugin.Logger(ctx).Debug(rateLimitLogString("github_release", &query.RateLimit))
	if err != nil {
		plugin.Logger(ctx).Error("github_release", "api_error", err)
		return nil, err
	}

	return query.Repository.Release, nil
}
