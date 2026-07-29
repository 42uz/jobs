package source

import (
	"context"
	"fmt"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// smartrecruiters adapts api.smartrecruiters.com public postings. The list
// endpoint does not include descriptions, so jobs link out for full details.
type smartrecruiters struct{}

func init() { Register(smartrecruiters{}) }

func (smartrecruiters) Kind() string { return "smartrecruiters" }

type srResponse struct {
	Offset     int         `json:"offset"`
	Limit      int         `json:"limit"`
	TotalFound int         `json:"totalFound"`
	Content    []srPosting `json:"content"`
}

type srPosting struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	RefNumber    string `json:"refNumber"`
	ReleasedDate string `json:"releasedDate"`
	Company      struct {
		Identifier string `json:"identifier"`
		Name       string `json:"name"`
	} `json:"company"`
	Location struct {
		City         string `json:"city"`
		Region       string `json:"region"`
		Country      string `json:"country"`
		FullLocation string `json:"fullLocation"`
		Remote       bool   `json:"remote"`
	} `json:"location"`
	Department struct {
		Label string `json:"label"`
	} `json:"department"`
	Function struct {
		Label string `json:"label"`
	} `json:"function"`
}

func (smartrecruiters) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	const pageSize = 100
	maxJobs := configInt(c, "maxJobs", 6000)

	var jobs []model.Job
	for offset := 0; offset < maxJobs; offset += pageSize {
		url := fmt.Sprintf("https://api.smartrecruiters.com/v1/companies/%s/postings?limit=%d&offset=%d", c.Slug, pageSize, offset)
		var resp srResponse
		if err := f.GetJSON(ctx, url, nil, &resp); err != nil {
			if len(jobs) > 0 {
				break // keep what we already gathered
			}
			return nil, err
		}
		if len(resp.Content) == 0 {
			break
		}
		for _, p := range resp.Content {
			loc := firstNonEmpty(p.Location.FullLocation, joinLoc(p.Location.City, p.Location.Region, p.Location.Country))
			jobs = append(jobs, model.Job{
				ID:         c.ID + "~" + model.StableID(p.ID),
				Company:    firstNonEmpty(p.Company.Name, c.Name),
				Title:      p.Name,
				Location:   loc,
				Department: firstNonEmpty(p.Department.Label, p.Function.Label),
				Remote:     p.Location.Remote,
				URL:        fmt.Sprintf("https://jobs.smartrecruiters.com/%s/%s", c.Slug, p.ID),
				PostedAt:   parseTime(p.ReleasedDate),
				UpdatedAt:  parseTime(p.ReleasedDate),
			})
		}
		if offset+len(resp.Content) >= resp.TotalFound {
			break
		}
	}
	return finalizeResult(c, jobs)
}

func joinLoc(parts ...string) string {
	out := ""
	for _, p := range parts {
		if p == "" {
			continue
		}
		if out != "" {
			out += ", "
		}
		out += p
	}
	return out
}
