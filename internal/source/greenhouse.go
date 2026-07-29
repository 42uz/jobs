package source

import (
	"context"
	"fmt"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// greenhouse adapts boards-api.greenhouse.io. A single host serves every
// Greenhouse company, so per-host pacing in the Fetcher is important here.
type greenhouse struct{}

func init() { Register(greenhouse{}) }

func (greenhouse) Kind() string { return "greenhouse" }

type ghResponse struct {
	Jobs []ghJob `json:"jobs"`
}

type ghJob struct {
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	AbsoluteURL string `json:"absolute_url"`
	UpdatedAt   string `json:"updated_at"`
	Published   string `json:"first_published"`
	Content     string `json:"content"`
	CompanyName string `json:"company_name"`
	Location    struct {
		Name string `json:"name"`
	} `json:"location"`
	Departments []struct {
		Name string `json:"name"`
	} `json:"departments"`
	Offices []struct {
		Name string `json:"name"`
	} `json:"offices"`
}

func (greenhouse) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	url := fmt.Sprintf("https://boards-api.greenhouse.io/v1/boards/%s/jobs?content=true", c.Slug)
	var resp ghResponse
	if err := f.GetJSON(ctx, url, nil, &resp); err != nil {
		return nil, err
	}
	jobs := make([]model.Job, 0, len(resp.Jobs))
	for _, j := range resp.Jobs {
		dept := ""
		if len(j.Departments) > 0 {
			dept = j.Departments[0].Name
		}
		var locs []string
		for _, o := range j.Offices {
			if o.Name != "" {
				locs = append(locs, o.Name)
			}
		}
		jobs = append(jobs, model.Job{
			ID:          c.ID + "~" + model.StableID(fmt.Sprint(j.ID)),
			Company:     firstNonEmpty(j.CompanyName, c.Name),
			Title:       j.Title,
			Location:    j.Location.Name,
			Locations:   locs,
			Department:  dept,
			URL:         j.AbsoluteURL,
			Description: unescapeAndClean(j.Content),
			PostedAt:    parseTime(j.Published),
			UpdatedAt:   parseTime(j.UpdatedAt),
		})
	}
	return finalizeResult(c, jobs)
}
