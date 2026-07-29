package source

import (
	"context"
	"fmt"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// ashby adapts api.ashbyhq.com posting-api job boards.
type ashby struct{}

func init() { Register(ashby{}) }

func (ashby) Kind() string { return "ashby" }

type ashbyResponse struct {
	Jobs []ashbyJob `json:"jobs"`
}

type ashbyJob struct {
	ID                 string `json:"id"`
	Title              string `json:"title"`
	Department         string `json:"department"`
	Team               string `json:"team"`
	Location           string `json:"location"`
	SecondaryLocations []struct {
		Location string `json:"location"`
	} `json:"secondaryLocations"`
	PublishedAt     string `json:"publishedAt"`
	IsListed        bool   `json:"isListed"`
	IsRemote        bool   `json:"isRemote"`
	JobURL          string `json:"jobUrl"`
	ApplyURL        string `json:"applyUrl"`
	DescriptionHTML string `json:"descriptionHtml"`
}

func (ashby) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	url := fmt.Sprintf("https://api.ashbyhq.com/posting-api/job-board/%s?includeCompensation=false", c.Slug)
	var resp ashbyResponse
	if err := f.GetJSON(ctx, url, nil, &resp); err != nil {
		return nil, err
	}
	jobs := make([]model.Job, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		if !j.IsListed {
			continue
		}
		locs := []string{j.Location}
		for _, s := range j.SecondaryLocations {
			if s.Location != "" {
				locs = append(locs, s.Location)
			}
		}
		jobs = append(jobs, model.Job{
			ID:          c.ID + "~" + model.StableID(j.ID),
			Title:       j.Title,
			Location:    j.Location,
			Locations:   locs,
			Department:  firstNonEmpty(j.Department, j.Team),
			Remote:      j.IsRemote,
			URL:         firstNonEmpty(j.JobURL, j.ApplyURL),
			Description: sanitizeHTML(j.DescriptionHTML),
			PostedAt:    parseTime(j.PublishedAt),
			UpdatedAt:   parseTime(j.PublishedAt),
		})
	}
	return finalizeResult(c, jobs)
}
