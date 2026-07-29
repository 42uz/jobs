package source

import (
	"context"
	"fmt"
	"strings"

	"faangjobs/internal/model"
	"faangjobs/internal/registry"
)

// lever adapts api.lever.co (v0 postings API).
type lever struct{}

func init() { Register(lever{}) }

func (lever) Kind() string { return "lever" }

type leverJob struct {
	ID            string   `json:"id"`
	Text          string   `json:"text"`
	HostedURL     string   `json:"hostedUrl"`
	ApplyURL      string   `json:"applyUrl"`
	CreatedAt     int64    `json:"createdAt"`
	Description   string   `json:"description"`
	WorkplaceType string   `json:"workplaceType"`
	Country       string   `json:"country"`
	AllLocations  []string `json:"allLocations"`
	Categories    struct {
		Commitment string `json:"commitment"`
		Department string `json:"department"`
		Location   string `json:"location"`
		Team       string `json:"team"`
	} `json:"categories"`
}

func (lever) Fetch(ctx context.Context, f *Fetcher, c registry.Company) ([]model.Job, error) {
	url := fmt.Sprintf("https://api.lever.co/v0/postings/%s?mode=json", c.Slug)
	var resp []leverJob
	if err := f.GetJSON(ctx, url, nil, &resp); err != nil {
		return nil, err
	}
	jobs := make([]model.Job, 0, len(resp))
	for _, j := range resp {
		dept := firstNonEmpty(j.Categories.Department, j.Categories.Team)
		remote := strings.EqualFold(j.WorkplaceType, "remote")
		jobs = append(jobs, model.Job{
			ID:          c.ID + "~" + model.StableID(j.ID),
			Title:       j.Text,
			Location:    firstNonEmpty(j.Categories.Location, strings.Join(j.AllLocations, ", ")),
			Locations:   j.AllLocations,
			Department:  dept,
			Remote:      remote,
			URL:         firstNonEmpty(j.HostedURL, j.ApplyURL),
			Description: sanitizeHTML(j.Description),
			PostedAt:    parseEpoch(j.CreatedAt),
			UpdatedAt:   parseEpoch(j.CreatedAt),
		})
	}
	return finalizeResult(c, jobs)
}
