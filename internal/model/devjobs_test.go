package model

import "testing"

func TestIsDevJob(t *testing.T) {
	dev := []Job{
		{Title: "Senior Software Engineer", Categories: Categorize("Senior Software Engineer", "")},
		{Title: "Machine Learning Engineer", Categories: Categorize("Machine Learning Engineer", "")},
		{Title: "Site Reliability Engineer", Categories: Categorize("Site Reliability Engineer", "")},
		{Title: "Security Engineer", Categories: Categorize("Security Engineer", "")},
		{Title: "QA Engineer", Categories: Categorize("QA Engineer", "")},
		{Title: "Firmware Developer", Categories: Categorize("Firmware Developer", "")},
		{Title: "IT Support Specialist", Categories: Categorize("IT Support Specialist", "")},
		{Title: "System Administrator", Categories: Categorize("System Administrator", "")},
		{Title: "Backend Developer", Categories: Categorize("Backend Developer", "")},
	}
	for _, j := range dev {
		if !IsDevJob(j) {
			t.Errorf("expected dev job: %q (cats %v)", j.Title, j.Categories)
		}
	}
	nonDev := []Job{
		{Title: "Account Executive", Categories: Categorize("Account Executive", "Sales")},
		{Title: "Technical Recruiter", Categories: Categorize("Technical Recruiter", "People")},
		{Title: "Product Designer", Categories: Categorize("Product Designer", "Design")},
		{Title: "Marketing Manager", Categories: Categorize("Marketing Manager", "")},
		{Title: "Corporate Counsel", Categories: Categorize("Corporate Counsel", "Legal")},
		{Title: "Warehouse Operations Associate", Categories: Categorize("Warehouse Operations Associate", "")},
	}
	for _, j := range nonDev {
		if IsDevJob(j) {
			t.Errorf("did NOT expect dev job: %q (cats %v)", j.Title, j.Categories)
		}
	}
}
