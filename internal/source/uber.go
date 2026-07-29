package source

import (
	"context"
	"fmt"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// uber adapts Uber's official careers API (www.uber.com/careers). The endpoint
// requires an x-csrf-token header, which accepts the literal placeholder "x"
// for unauthenticated search.
type uber struct{}

func init() { Register(uber{}) }

func (uber) Kind() string { return "uber" }

type uberResponse struct {
	Data struct {
		Results      []uberJob `json:"results"`
		TotalResults struct {
			Low int `json:"low"`
		} `json:"totalResults"`
	} `json:"data"`
}

type uberLocation struct {
	City        string `json:"city"`
	Region      string `json:"region"`
	CountryName string `json:"countryName"`
}

type uberJob struct {
	ID           int64          `json:"id"`
	Title        string         `json:"title"`
	Description  string         `json:"description"`
	Department   string         `json:"department"`
	CreationDate string         `json:"creationDate"`
	UpdatedDate  string         `json:"updatedDate"`
	Location     uberLocation   `json:"location"`
	AllLocations []uberLocation `json:"allLocations"`
}

func (l uberLocation) display() string {
	return joinLoc(l.City, l.Region, l.CountryName)
}

func (uber) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	const pageSize = 50
	maxJobs := configInt(c, "maxJobs", 6000)
	endpoint := "https://www.uber.com/api/loadSearchJobsResults?localeCode=en"
	headers := map[string]string{"x-csrf-token": "x"}

	var jobs []model.Job
	for page := 0; len(jobs) < maxJobs; page++ {
		body := map[string]any{
			"params": map[string]any{},
			"limit":  pageSize,
			"page":   page,
		}
		var resp uberResponse
		if err := f.PostJSON(ctx, endpoint, body, cloneHeaders(headers), &resp); err != nil {
			if len(jobs) > 0 {
				break
			}
			return nil, err
		}
		results := resp.Data.Results
		if len(results) == 0 {
			break
		}
		for _, j := range results {
			locs := make([]string, 0, len(j.AllLocations)+1)
			primary := j.Location.display()
			if primary != "" {
				locs = append(locs, primary)
			}
			for _, l := range j.AllLocations {
				if d := l.display(); d != "" && d != primary {
					locs = append(locs, d)
				}
			}
			jobs = append(jobs, model.Job{
				ID:          c.ID + "~" + model.StableID(fmt.Sprint(j.ID)),
				Title:       j.Title,
				Location:    primary,
				Locations:   locs,
				Department:  j.Department,
				URL:         fmt.Sprintf("https://www.uber.com/global/en/careers/list/%d/", j.ID),
				Description: sanitizeHTML(j.Description),
				PostedAt:    parseTime(j.CreationDate),
				UpdatedAt:   parseTime(firstNonEmpty(j.UpdatedDate, j.CreationDate)),
			})
		}
		if total := resp.Data.TotalResults.Low; total > 0 && (page+1)*pageSize >= total {
			break
		}
	}
	return finalizeResult(c, jobs)
}
