package pdf

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
)

// validator is a veraPDF REST service (the verapdf preset): POST a document
// to /api/validate/{profile} and read the rule violations back.
type validator struct {
	baseURL string
	client  *http.Client
}

// profile is a conformance level people name in a step, and veraPDF's id
// for it.
type profile struct {
	name string
	id   string
}

// profiles are the validation profiles veraPDF 1.30 serves
// (/api/profiles/ids), by the name of the standard.
var profiles = []profile{
	{"PDF/UA-1", "ua1"}, {"PDF/UA-2", "ua2"},
	{"PDF/A-1a", "1a"}, {"PDF/A-1b", "1b"},
	{"PDF/A-2a", "2a"}, {"PDF/A-2b", "2b"}, {"PDF/A-2u", "2u"},
	{"PDF/A-3a", "3a"}, {"PDF/A-3b", "3b"}, {"PDF/A-3u", "3u"},
	{"PDF/A-4", "4"}, {"PDF/A-4e", "4e"}, {"PDF/A-4f", "4f"},
	{"WTPDF-1.0-Reuse", "wt1r"}, {"WTPDF-1.0-Accessibility", "wt1a"},
}

// profileFor resolves a name like "PDF/UA-1" (or veraPDF's own "ua1") to a
// profile.
func profileFor(name string) (profile, error) {
	want := strings.ToLower(strings.TrimSpace(name))
	for _, p := range profiles {
		if want == strings.ToLower(p.name) || want == p.id {
			return p, nil
		}
	}
	names := make([]string, len(profiles))
	for i, p := range profiles {
		names[i] = p.name
	}
	return profile{}, fmt.Errorf("unknown conformance level %q; one of: %s", name, strings.Join(names, ", "))
}

// violation is one rule the document breaks.
type violation struct {
	Clause      string
	Test        int
	Description string
	Checks      int
	Context     string
}

// conformance is veraPDF's verdict.
type conformance struct {
	Compliant   bool
	ProfileName string
	Violations  []violation
}

// The REST service's report, the parts tomato reads. validationResult is
// an array in 1.26 and later and an object before, so it is decoded in two
// steps.
type veraReport struct {
	Report struct {
		Jobs []struct {
			ValidationResult json.RawMessage `json:"validationResult"`
			TaskResult       *struct {
				Type             string `json:"type"`
				IsSuccess        bool   `json:"isSuccess"`
				ExceptionMessage string `json:"exceptionMessage"`
			} `json:"taskResult"`
		} `json:"jobs"`
	} `json:"report"`
}

type veraResult struct {
	Compliant   bool   `json:"compliant"`
	ProfileName string `json:"profileName"`
	Details     struct {
		FailedRules   int `json:"failedRules"`
		RuleSummaries []struct {
			Clause       string `json:"clause"`
			TestNumber   int    `json:"testNumber"`
			Status       string `json:"status"`
			FailedChecks int    `json:"failedChecks"`
			Description  string `json:"description"`
			Checks       []struct {
				Status       string `json:"status"`
				Context      string `json:"context"`
				ErrorMessage string `json:"errorMessage"`
			} `json:"checks"`
		} `json:"ruleSummaries"`
	} `json:"details"`
}

// ping checks the service answers.
func (v *validator) ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.baseURL+"/api/info", nil)
	if err != nil {
		return err
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s/api/info: %s", v.baseURL, resp.Status)
	}
	return nil
}

// validate sends the document and reads the verdict.
func (v *validator) validate(ctx context.Context, data []byte, p profile) (conformance, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="document.pdf"`)
	h.Set("Content-Type", "application/pdf")
	part, err := w.CreatePart(h)
	if err != nil {
		return conformance{}, err
	}
	if _, err := part.Write(data); err != nil {
		return conformance{}, err
	}
	if err := w.Close(); err != nil {
		return conformance{}, err
	}

	url := v.baseURL + "/api/validate/" + p.id
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, &body)
	if err != nil {
		return conformance{}, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	resp, err := v.client.Do(req)
	if err != nil {
		return conformance{}, fmt.Errorf("veraPDF: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
	if err != nil {
		return conformance{}, fmt.Errorf("veraPDF: reading the report: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return conformance{}, fmt.Errorf("veraPDF: POST %s: %s: %s", url, resp.Status, excerpt(raw))
	}
	return parseReport(raw)
}

func parseReport(raw []byte) (conformance, error) {
	var rep veraReport
	if err := json.Unmarshal(raw, &rep); err != nil {
		return conformance{}, fmt.Errorf("veraPDF: the report is not JSON: %w: %s", err, excerpt(raw))
	}
	if len(rep.Report.Jobs) == 0 {
		return conformance{}, fmt.Errorf("veraPDF: the report has no job: %s", excerpt(raw))
	}
	job := rep.Report.Jobs[0]
	var results []veraResult
	if len(job.ValidationResult) > 0 && job.ValidationResult[0] == '[' {
		if err := json.Unmarshal(job.ValidationResult, &results); err != nil {
			return conformance{}, fmt.Errorf("veraPDF: reading the validation result: %w", err)
		}
	} else if len(job.ValidationResult) > 0 && job.ValidationResult[0] == '{' {
		var one veraResult
		if err := json.Unmarshal(job.ValidationResult, &one); err != nil {
			return conformance{}, fmt.Errorf("veraPDF: reading the validation result: %w", err)
		}
		results = []veraResult{one}
	}
	if len(results) == 0 {
		if job.TaskResult != nil && job.TaskResult.ExceptionMessage != "" {
			return conformance{}, fmt.Errorf("veraPDF could not validate the document (%s): %s", strings.ToLower(job.TaskResult.Type), job.TaskResult.ExceptionMessage)
		}
		return conformance{}, fmt.Errorf("veraPDF returned no validation result: %s", excerpt(raw))
	}
	r := results[0]
	c := conformance{Compliant: r.Compliant, ProfileName: r.ProfileName}
	for _, s := range r.Details.RuleSummaries {
		if !strings.EqualFold(s.Status, "failed") && s.FailedChecks == 0 {
			continue
		}
		v := violation{Clause: s.Clause, Test: s.TestNumber, Description: s.Description, Checks: s.FailedChecks}
		if len(s.Checks) > 0 {
			v.Context = s.Checks[0].Context
			if s.Checks[0].ErrorMessage != "" {
				v.Context = s.Checks[0].ErrorMessage + " at " + v.Context
			}
		}
		c.Violations = append(c.Violations, v)
	}
	return c, nil
}

// maxViolationsShown bounds a failure message; the first rules say enough.
const maxViolationsShown = 20

// describe formats the violations for a failed step.
func (c conformance) describe() string {
	var sb strings.Builder
	n := len(c.Violations)
	fmt.Fprintf(&sb, "%d rule%s failed (%s)", n, plural(n), c.ProfileName)
	for i, v := range c.Violations {
		if i == maxViolationsShown {
			fmt.Fprintf(&sb, "\n  … and %d more", n-i)
			break
		}
		fmt.Fprintf(&sb, "\n  %s-%d (%d check%s): %s", v.Clause, v.Test, v.Checks, plural(v.Checks), v.Description)
		if v.Context != "" {
			fmt.Fprintf(&sb, "\n      at %s", v.Context)
		}
	}
	return sb.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

func excerpt(b []byte) string {
	s := strings.TrimSpace(string(b))
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
