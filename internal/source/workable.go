package source

import (
	"context"
	"fmt"
	"strings"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// workable adapts the public apply.workable.com v3 accounts jobs API.
type workable struct{}

func init() { Register(workable{}) }

func (workable) Kind() string { return "workable" }

type workableResponse struct {
	Results []workableJob `json:"results"`
	Paging  struct {
		Next string `json:"next"`
	} `json:"paging"`
}

type workableJob struct {
	ID             string `json:"id"`
	Shortcode      string `json:"shortcode"`
	Title          string `json:"title"`
	EmploymentType string `json:"employment_type"`
	Department     string `json:"department"`
	URL            string `json:"url"`
	ApplicationURL string `json:"application_url"`
	Shortlink      string `json:"shortlink"`
	CreatedAt      string `json:"created_at"`
	Remote         bool   `json:"remote"`
	Location       struct {
		City        string `json:"city"`
		Region      string `json:"region"`
		Country     string `json:"country"`
		CountryCode string `json:"country_code"`
		Workplace   string `json:"workplace_type"`
	} `json:"location"`
}

func (workable) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	endpoint := fmt.Sprintf("https://apply.workable.com/api/v3/accounts/%s/jobs", c.Slug)
	var jobs []model.Job
	token := ""
	for page := 0; page < 60; page++ { // hard page cap
		body := map[string]any{}
		if token != "" {
			body["token"] = token
		}
		var resp workableResponse
		if err := f.PostJSON(ctx, endpoint, body, nil, &resp); err != nil {
			if len(jobs) > 0 {
				break
			}
			return nil, err
		}
		if len(resp.Results) == 0 {
			break
		}
		for _, j := range resp.Results {
			remote := j.Remote || strings.EqualFold(j.Location.Workplace, "remote")
			jobs = append(jobs, model.Job{
				ID:         c.ID + "~" + model.StableID(firstNonEmpty(j.Shortcode, j.ID)),
				Title:      j.Title,
				Location:   joinLoc(j.Location.City, j.Location.Region, firstNonEmpty(j.Location.Country, j.Location.CountryCode)),
				Department: j.Department,
				Remote:     remote,
				URL:        firstNonEmpty(j.URL, j.ApplicationURL, j.Shortlink),
				PostedAt:   parseTime(j.CreatedAt),
				UpdatedAt:  parseTime(j.CreatedAt),
			})
		}
		if resp.Paging.Next == "" {
			break
		}
		token = resp.Paging.Next
	}
	return finalizeResult(c, jobs)
}
