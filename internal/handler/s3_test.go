package handler

import (
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/tomatool/tomato/internal/config"
)

func TestSplitPath(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantBucket string
		wantKey    string
		wantErr    bool
	}{
		{"simple", "uploads/hello.txt", "uploads", "hello.txt", false},
		{"nested key", "uploads/2026/09/report.pdf", "uploads", "2026/09/report.pdf", false},
		{"s3 scheme stripped", "s3://uploads/hello.txt", "uploads", "hello.txt", false},
		{"key with dots", "my-bucket/a.b.c.json", "my-bucket", "a.b.c.json", false},
		{"no separator", "uploads", "", "", true},
		{"empty key", "uploads/", "", "", true},
		{"empty bucket", "/hello.txt", "", "", true},
		{"empty string", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bucket, key, err := splitPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("splitPath(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if bucket != tt.wantBucket || key != tt.wantKey {
				t.Errorf("splitPath(%q) = (%q, %q), want (%q, %q)",
					tt.input, bucket, key, tt.wantBucket, tt.wantKey)
			}
		})
	}
}

func TestSplitPathResolvesVariables(t *testing.T) {
	ResetGlobalVariables()
	SetVariable("bucket_name", "dynamic")
	defer ResetGlobalVariables()

	bucket, key, err := splitPath("{{bucket_name}}/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if bucket != "dynamic" || key != "file.txt" {
		t.Errorf("got (%q, %q), want (\"dynamic\", \"file.txt\")", bucket, key)
	}
}

func TestWithScheme(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		ssl      bool
		want     string
	}{
		{"bare host adds http", "localhost:9000", false, "http://localhost:9000"},
		{"bare host adds https when ssl", "localhost:9000", true, "https://localhost:9000"},
		{"existing http preserved", "http://localhost:9000", false, "http://localhost:9000"},
		{"existing https preserved despite ssl false", "https://s3.example.com", false, "https://s3.example.com"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := withScheme(tt.endpoint, tt.ssl); got != tt.want {
				t.Errorf("withScheme(%q, %v) = %q, want %q", tt.endpoint, tt.ssl, got, tt.want)
			}
		})
	}
}

func TestIsNotFound(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"NoSuchKey", &types.NoSuchKey{}, true},
		{"NoSuchBucket", &types.NoSuchBucket{}, true},
		{"NotFound", &types.NotFound{}, true},
		{"api error NotFound", &smithy.GenericAPIError{Code: "NotFound"}, true},
		{"api error 404", &smithy.GenericAPIError{Code: "404"}, true},
		{"api error AccessDenied", &smithy.GenericAPIError{Code: "AccessDenied"}, false},
		{"plain error", errors.New("connection refused"), false},
		{"wrapped NoSuchKey", errors.Join(errors.New("ctx"), &types.NoSuchKey{}), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNotFound(tt.err); got != tt.want {
				t.Errorf("isNotFound(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestPoll(t *testing.T) {
	t.Run("passes immediately", func(t *testing.T) {
		calls := 0
		err := poll("1s", func() error { calls++; return nil })
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 1 {
			t.Errorf("check called %d times, want 1", calls)
		}
	})

	t.Run("passes after retries", func(t *testing.T) {
		calls := 0
		err := poll("2s", func() error {
			calls++
			if calls < 3 {
				return errors.New("not yet")
			}
			return nil
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if calls != 3 {
			t.Errorf("check called %d times, want 3", calls)
		}
	})

	t.Run("surfaces last error on timeout", func(t *testing.T) {
		start := time.Now()
		err := poll("300ms", func() error { return errors.New("object missing") })
		if err == nil {
			t.Fatal("expected timeout error")
		}
		if !contains(err.Error(), "object missing") {
			t.Errorf("error %q should wrap the last failure", err)
		}
		if elapsed := time.Since(start); elapsed < 300*time.Millisecond {
			t.Errorf("returned after %v, should have waited the full 300ms", elapsed)
		}
	})

	t.Run("rejects invalid duration", func(t *testing.T) {
		if err := poll("soon", func() error { return nil }); err == nil {
			t.Error("expected an error for an unparseable duration")
		}
	})
}

func TestS3OptionParsing(t *testing.T) {
	h, err := NewS3("files", config.Resource{
		Type: "s3",
		Options: map[string]any{
			"region":        "eu-central-1",
			"buckets":       []interface{}{"uploads", "reports", 42},
			"reset_exclude": []interface{}{"keepme"},
		},
	}, nil)
	if err != nil {
		t.Fatalf("NewS3: %v", err)
	}

	if got := h.option("region", "us-east-1"); got != "eu-central-1" {
		t.Errorf("region = %q, want eu-central-1", got)
	}
	if got := h.option("access_key", "minioadmin"); got != "minioadmin" {
		t.Errorf("access_key = %q, want the fallback minioadmin", got)
	}

	buckets := h.configuredBuckets()
	if len(buckets) != 2 || buckets[0] != "uploads" || buckets[1] != "reports" {
		t.Errorf("configuredBuckets() = %v, want [uploads reports] with the non-string dropped", buckets)
	}
	if excl := h.optionList("reset_exclude"); len(excl) != 1 || excl[0] != "keepme" {
		t.Errorf("reset_exclude = %v, want [keepme]", excl)
	}
	if missing := h.optionList("nope"); missing != nil {
		t.Errorf("optionList on a missing key = %v, want nil", missing)
	}
}

func TestS3ResolveEndpointRequiresContainerOrEndpoint(t *testing.T) {
	h, _ := NewS3("files", config.Resource{Type: "s3"}, nil)
	if _, err := h.resolveEndpoint(t.Context()); err == nil {
		t.Error("expected an error when neither endpoint nor container is configured")
	}

	h, _ = NewS3("files", config.Resource{
		Type:    "s3",
		Options: map[string]any{"endpoint": "localhost:4566"},
	}, nil)
	got, err := h.resolveEndpoint(t.Context())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "http://localhost:4566" {
		t.Errorf("resolveEndpoint() = %q, want http://localhost:4566", got)
	}
}

func TestS3StepsAreWellFormed(t *testing.T) {
	h, _ := NewS3("files", DummyConfig(), nil)
	cat := h.Steps()

	if cat.Name != "S3" {
		t.Errorf("category name = %q, want S3", cat.Name)
	}
	if len(cat.Steps) == 0 {
		t.Fatal("no steps registered")
	}

	seen := make(map[string]bool)
	for _, step := range cat.Steps {
		if step.Pattern == "" || step.Description == "" || step.Handler == nil {
			t.Errorf("step %q is missing a pattern, description, or handler", step.Pattern)
		}
		if step.Group == "" {
			t.Errorf("step %q has no group, so it will not render in the docs", step.Pattern)
		}
		if !contains(step.Pattern, "{resource}") {
			t.Errorf("step %q does not use the {resource} placeholder", step.Pattern)
		}
		if seen[step.Pattern] {
			t.Errorf("duplicate step pattern %q", step.Pattern)
		}
		seen[step.Pattern] = true
	}
}
