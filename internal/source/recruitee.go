package source

import (
	"context"
	"fmt"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// recruitee adapts the public {slug}.recruitee.com offers API.
type recruitee struct{}

func init() { Register(recruitee{}) }

func (recruitee) Kind() string { return "recruitee" }

type recruiteeResponse struct {
	Offers []recruiteeOffer `json:"offers"`
}

type recruiteeOffer struct {
	ID           int64  `json:"id"`
	Title        string `json:"title"`
	Slug         string `json:"slug"`
	CareersURL   string `json:"careers_url"`
	Location     string `json:"location"`
	City         string `json:"city"`
	CountryCode  string `json:"country_code"`
	Department   string `json:"department"`
	Description  string `json:"description"`
	Requirements string `json:"requirements"`
	CreatedAt    string `json:"created_at"`
	Remote       bool   `json:"remote"`
}

func (recruitee) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	url := fmt.Sprintf("https://%s.recruitee.com/api/offers/", c.Slug)
	var resp recruiteeResponse
	if err := f.GetJSON(ctx, url, nil, &resp); err != nil {
		return nil, err
	}
	jobs := make([]model.Job, 0, len(resp.Offers))
	for _, o := range resp.Offers {
		desc := o.Description
		if o.Requirements != "" {
			desc += "<h3>Requirements</h3>" + o.Requirements
		}
		jobs = append(jobs, model.Job{
			ID:          c.ID + "~" + model.StableID(fmt.Sprint(o.ID)),
			Title:       o.Title,
			Location:    firstNonEmpty(o.Location, joinLoc(o.City, o.CountryCode)),
			Department:  o.Department,
			Remote:      o.Remote,
			URL:         o.CareersURL,
			Description: sanitizeHTML(desc),
			PostedAt:    parseTime(o.CreatedAt),
			UpdatedAt:   parseTime(o.CreatedAt),
		})
	}
	return finalizeResult(c, jobs)
}
