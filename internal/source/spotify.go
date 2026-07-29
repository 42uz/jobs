package source

import (
	"context"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// spotify adapts Spotify's official careers site (lifeatspotify.com), which is
// backed by a public JSON search API. The listing has no dates or
// descriptions; jobs link to the official posting page.
type spotify struct{}

func init() { Register(spotify{}) }

func (spotify) Kind() string { return "spotify" }

type spotifyResponse struct {
	Result []spotifyJob `json:"result"`
}

type spotifyJob struct {
	ID           string `json:"id"` // slug, e.g. "android-engineer-advertising"
	Text         string `json:"text"`
	MainCategory struct {
		Name string `json:"name"`
	} `json:"main_category"`
	Locations []struct {
		Location string `json:"location"`
	} `json:"locations"`
}

func (spotify) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	const endpoint = "https://api.lifeatspotify.com/wp-json/animal/v1/job/search?l=&c=&q="
	var resp spotifyResponse
	if err := f.GetJSON(ctx, endpoint, nil, &resp); err != nil {
		return nil, err
	}
	jobs := make([]model.Job, 0, len(resp.Result))
	for _, j := range resp.Result {
		var locs []string
		for _, l := range j.Locations {
			if l.Location != "" {
				locs = append(locs, l.Location)
			}
		}
		primary := ""
		if len(locs) > 0 {
			primary = locs[0]
		}
		jobs = append(jobs, model.Job{
			ID:         c.ID + "~" + model.StableID(j.ID),
			Title:      j.Text,
			Location:   primary,
			Locations:  locs,
			Department: j.MainCategory.Name,
			URL:        "https://www.lifeatspotify.com/jobs/" + j.ID,
		})
	}
	return finalizeResult(c, jobs)
}
