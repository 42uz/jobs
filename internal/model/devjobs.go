package model

import "strings"

// This file implements the board's profession policy: keep only developer /
// IT roles (software engineers, programmers, data/ML, infra, security, QA,
// sysadmin — "IT related jobs").

// devCategories are the Categorize() outputs that always qualify.
var devCategories = map[string]bool{
	"Software Engineering":    true,
	"Data & ML":               true,
	"Infrastructure & DevOps": true,
	"Security":                true,
}

// devExtraKeywords is a safety net for IT-ish roles whose category lands
// elsewhere (e.g. QA under Other, firmware under Hardware, IT support under
// Support).
var devExtraKeywords = []string{
	"information technology", "it support", "it engineer", "it specialist",
	"it administrator", "it analyst", "system administrator", "sysadmin",
	"database administrator", "dba ", "qa engineer", "quality assurance",
	"test engineer", "test automation", "automation engineer", "firmware",
	"embedded", "sre", "site reliability", "devops", "data engineer",
	"solutions architect", "cloud architect", "software architect",
}

// IsDevJob reports whether a job is a developer / IT role under the board's
// profession policy.
func IsDevJob(j Job) bool {
	for _, c := range j.Categories {
		if devCategories[c] {
			return true
		}
	}
	hay := strings.ToLower(j.Title)
	for _, kw := range devExtraKeywords {
		if strings.Contains(hay, kw) {
			return true
		}
	}
	return false
}

// DevJob is the transform/keep form of IsDevJob, composable with the region
// policy in the crawler's Filter chain.
func DevJob(j Job) (Job, bool) {
	return j, IsDevJob(j)
}
