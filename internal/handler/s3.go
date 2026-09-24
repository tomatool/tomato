package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	"github.com/cucumber/godog"
	"github.com/tomatool/tomato/internal/config"
	"github.com/tomatool/tomato/internal/container"
)

// defaultS3Ports are probed in order when no explicit port option is set.
// 9000 is MinIO's API port, 4566 is the LocalStack edge port.
var defaultS3Ports = []string{"9000/tcp", "4566/tcp"}

// S3 provides object storage testing capabilities against any S3-compatible
// endpoint (MinIO, LocalStack, or a real bucket).
type S3 struct {
	name      string
	config    config.Resource
	container *container.Manager
	client    *s3.Client
	skipReset bool // remote target without `reset: true`
}

func NewS3(name string, cfg config.Resource, cm *container.Manager) (*S3, error) {
	return &S3{name: name, config: cfg, container: cm}, nil
}

func (r *S3) Name() string { return r.name }

func (r *S3) Init(ctx context.Context) error {
	endpoint, err := r.resolveEndpoint(ctx)
	if err != nil {
		return err
	}
	r.skipReset = remoteResetGuard(r.name, r.config, endpoint)

	accessKey := r.option("access_key", "minioadmin")
	secretKey := r.option("secret_key", "minioadmin")
	region := r.option("region", "us-east-1")

	pathStyle := true
	if v, ok := r.config.Options["force_path_style"].(bool); ok {
		pathStyle = v
	}

	r.client = s3.NewFromConfig(aws.Config{
		Region:      region,
		Credentials: credentials.NewStaticCredentialsProvider(accessKey, secretKey, ""),
	}, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = pathStyle
	})

	// Pre-create any declared buckets so feature files can assume they exist.
	for _, b := range r.configuredBuckets() {
		if err := r.ensureBucket(ctx, b); err != nil {
			return fmt.Errorf("creating bucket %q: %w", b, err)
		}
	}
	return nil
}

// resolveEndpoint prefers an explicit endpoint/base_url, otherwise derives one
// from the referenced container by probing the known S3 API ports.
func (r *S3) resolveEndpoint(ctx context.Context) (string, error) {
	if ep := r.option("endpoint", r.config.BaseURL); ep != "" {
		return withScheme(ep, r.useSSL()), nil
	}

	if r.config.Container == "" {
		return "", errors.New("s3 resource needs either options.endpoint or a container reference")
	}

	host, err := r.container.GetHost(ctx, r.config.Container)
	if err != nil {
		return "", fmt.Errorf("getting container host: %w", err)
	}

	ports := defaultS3Ports
	if p := r.option("port", ""); p != "" {
		if !strings.Contains(p, "/") {
			p += "/tcp"
		}
		ports = []string{p}
	}

	var lastErr error
	for _, p := range ports {
		port, err := r.container.GetPort(ctx, r.config.Container, p)
		if err != nil {
			lastErr = err
			continue
		}
		return withScheme(fmt.Sprintf("%s:%s", host, port), r.useSSL()), nil
	}
	return "", fmt.Errorf("container %q exposes none of %v: %w", r.config.Container, ports, lastErr)
}

func (r *S3) useSSL() bool {
	v, _ := r.config.Options["use_ssl"].(bool)
	return v
}

func withScheme(endpoint string, ssl bool) string {
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	if ssl {
		return "https://" + endpoint
	}
	return "http://" + endpoint
}

func (r *S3) option(key, fallback string) string {
	if v, ok := r.config.Options[key].(string); ok && v != "" {
		return v
	}
	return fallback
}

func (r *S3) optionList(key string) []string {
	raw, ok := r.config.Options[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func (r *S3) configuredBuckets() []string { return r.optionList("buckets") }

func (r *S3) Ready(ctx context.Context) error {
	_, err := r.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	return err
}

// Reset returns the endpoint to a clean state between scenarios.
//
// Strategies:
//   - purge (default): delete every object in the managed buckets, keep buckets
//   - delete: drop the managed buckets entirely, then recreate declared ones
//   - none: leave storage untouched
func (r *S3) Reset(ctx context.Context) error {
	strategy := r.option("reset_strategy", "purge")
	if strategy == "none" || r.skipReset {
		return nil
	}

	buckets, err := r.bucketsToReset(ctx)
	if err != nil {
		return err
	}

	for _, b := range buckets {
		if err := r.emptyBucketCtx(ctx, b); err != nil {
			return fmt.Errorf("emptying bucket %q: %w", b, err)
		}
		if strategy == "delete" {
			if _, err := r.client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(b)}); err != nil {
				return fmt.Errorf("deleting bucket %q: %w", b, err)
			}
		}
	}

	if strategy == "delete" {
		for _, b := range r.configuredBuckets() {
			if err := r.ensureBucket(ctx, b); err != nil {
				return fmt.Errorf("recreating bucket %q: %w", b, err)
			}
		}
	}
	return nil
}

// bucketsToReset defaults to every bucket on the endpoint so that buckets the
// application creates on its own are still cleaned up. reset_buckets narrows
// that to an explicit list; reset_exclude removes individual buckets from it.
func (r *S3) bucketsToReset(ctx context.Context) ([]string, error) {
	excluded := r.optionList("reset_exclude")

	candidates := r.optionList("reset_buckets")
	if len(candidates) == 0 {
		out, err := r.client.ListBuckets(ctx, &s3.ListBucketsInput{})
		if err != nil {
			return nil, fmt.Errorf("listing buckets: %w", err)
		}
		for _, b := range out.Buckets {
			candidates = append(candidates, aws.ToString(b.Name))
		}
	}

	var buckets []string
	for _, name := range candidates {
		if !containsString(excluded, name) {
			buckets = append(buckets, name)
		}
	}
	return buckets, nil
}

func containsString(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func (r *S3) Cleanup(ctx context.Context) error { return nil }

func (r *S3) RegisterSteps(ctx *godog.ScenarioContext) {
	RegisterStepsToGodog(ctx, r.name, r.Steps())
}

// Steps returns the structured step definitions for the S3 handler
func (r *S3) Steps() StepCategory {
	return StepCategory{
		Name:        "S3",
		Description: "Steps for interacting with S3-compatible object storage (MinIO, LocalStack, AWS)",
		Steps: []StepDef{
			// Bucket Setup
			{
				Group:       "Bucket Setup",
				Pattern:     `^"{resource}" bucket "([^"]*)" exists$`,
				Description: "Create a bucket if it does not already exist",
				Example:     `"files" bucket "uploads" exists`,
				Handler:     r.bucketExists,
			},
			{
				Group:       "Bucket Setup",
				Pattern:     `^"{resource}" bucket "([^"]*)" is empty$`,
				Description: "Delete every object in a bucket",
				Example:     `"files" bucket "uploads" is empty`,
				Handler:     r.emptyBucket,
			},
			{
				Group:       "Bucket Setup",
				Pattern:     `^"{resource}" bucket "([^"]*)" is deleted$`,
				Description: "Delete a bucket and everything in it",
				Example:     `"files" bucket "uploads" is deleted`,
				Handler:     r.deleteBucket,
			},

			// Object Setup
			{
				Group:       "Object Setup",
				Pattern:     `^"{resource}" object "([^"]*)" is "([^"]*)"$`,
				Description: "Write an object with inline content (path is bucket/key)",
				Example:     `"files" object "uploads/hello.txt" is "hello world"`,
				Handler:     r.putObject,
			},
			{
				Group:       "Object Setup",
				Pattern:     `^"{resource}" object "([^"]*)" is:$`,
				Description: "Write an object with multiline content (docstring)",
				Example: `"files" object "uploads/user.json" is:
  """
  {"id": 1, "name": "John"}
  """`,
				Handler: r.putObjectDoc,
			},
			{
				Group:       "Object Setup",
				Pattern:     `^"{resource}" object "([^"]*)" is "([^"]*)" with content type "([^"]*)"$`,
				Description: "Write an object with an explicit content type",
				Example:     `"files" object "uploads/a.csv" is "id,name" with content type "text/csv"`,
				Handler:     r.putObjectWithContentType,
			},
			{
				Group:       "Object Setup",
				Pattern:     `^"{resource}" object "([^"]*)" is file "([^"]*)"$`,
				Description: "Upload a local file as an object",
				Example:     `"files" object "uploads/logo.png" is file "testdata/logo.png"`,
				Handler:     r.putObjectFromFile,
			},
			{
				Group:       "Object Setup",
				Pattern:     `^"{resource}" object "([^"]*)" is deleted$`,
				Description: "Delete a single object",
				Example:     `"files" object "uploads/hello.txt" is deleted`,
				Handler:     r.deleteObject,
			},
			{
				Group:       "Object Setup",
				Pattern:     `^"{resource}" objects:$`,
				Description: "Seed multiple objects from a table with path and content columns",
				Example: `"files" objects:
  | path                | content     |
  | uploads/a.txt       | first       |
  | uploads/b.txt       | second      |`,
				Handler: r.putObjects,
			},

			// Bucket Assertions
			{
				Group:       "Bucket Assertions",
				Pattern:     `^"{resource}" bucket "([^"]*)" should exist$`,
				Description: "Assert a bucket exists",
				Example:     `"files" bucket "uploads" should exist`,
				Handler:     r.bucketShouldExist,
			},
			{
				Group:       "Bucket Assertions",
				Pattern:     `^"{resource}" bucket "([^"]*)" should not exist$`,
				Description: "Assert a bucket does not exist",
				Example:     `"files" bucket "temp" should not exist`,
				Handler:     r.bucketShouldNotExist,
			},
			{
				Group:       "Bucket Assertions",
				Pattern:     `^"{resource}" bucket "([^"]*)" should have "(\d+)" objects$`,
				Description: "Assert the object count in a bucket",
				Example:     `"files" bucket "uploads" should have "3" objects`,
				Handler:     r.bucketShouldHaveCount,
			},
			{
				Group:       "Bucket Assertions",
				Pattern:     `^"{resource}" bucket "([^"]*)" should be empty$`,
				Description: "Assert a bucket holds no objects",
				Example:     `"files" bucket "uploads" should be empty`,
				Handler:     r.bucketShouldBeEmpty,
			},
			{
				Group:       "Bucket Assertions",
				Pattern:     `^"{resource}" bucket "([^"]*)" should have "(\d+)" objects with prefix "([^"]*)"$`,
				Description: "Assert the object count under a key prefix",
				Example:     `"files" bucket "uploads" should have "2" objects with prefix "2026/"`,
				Handler:     r.bucketShouldHaveCountWithPrefix,
			},
			{
				Group:       "Bucket Assertions",
				Pattern:     `^"{resource}" bucket "([^"]*)" should have "(\d+)" objects within "([^"]*)"$`,
				Description: "Wait for a bucket to reach an object count (async writes)",
				Example:     `"files" bucket "uploads" should have "1" objects within "10s"`,
				Handler:     r.bucketShouldHaveCountWithin,
			},

			// Object Assertions
			{
				Group:       "Object Assertions",
				Pattern:     `^"{resource}" object "([^"]*)" should exist$`,
				Description: "Assert an object exists",
				Example:     `"files" object "uploads/hello.txt" should exist`,
				Handler:     r.objectShouldExist,
			},
			{
				Group:       "Object Assertions",
				Pattern:     `^"{resource}" object "([^"]*)" should not exist$`,
				Description: "Assert an object does not exist",
				Example:     `"files" object "uploads/gone.txt" should not exist`,
				Handler:     r.objectShouldNotExist,
			},
			{
				Group:       "Object Assertions",
				Pattern:     `^"{resource}" object "([^"]*)" should exist within "([^"]*)"$`,
				Description: "Wait for an object to appear (async writes)",
				Example:     `"files" object "uploads/report.pdf" should exist within "10s"`,
				Handler:     r.objectShouldExistWithin,
			},
			{
				Group:       "Object Assertions",
				Pattern:     `^"{resource}" object "([^"]*)" content should be "([^"]*)"$`,
				Description: "Assert exact object content",
				Example:     `"files" object "uploads/hello.txt" content should be "hello world"`,
				Handler:     r.objectContentShouldBe,
			},
			{
				Group:       "Object Assertions",
				Pattern:     `^"{resource}" object "([^"]*)" content should be:$`,
				Description: "Assert exact object content (docstring)",
				Example: `"files" object "uploads/hello.txt" content should be:
  """
  hello world
  """`,
				Handler: r.objectContentShouldBeDoc,
			},
			{
				Group:       "Object Assertions",
				Pattern:     `^"{resource}" object "([^"]*)" content should contain "([^"]*)"$`,
				Description: "Assert object content contains a substring",
				Example:     `"files" object "uploads/log.txt" content should contain "ERROR"`,
				Handler:     r.objectContentShouldContain,
			},
			{
				Group:       "Object Assertions",
				Pattern:     `^"{resource}" object "([^"]*)" content should match:$`,
				Description: "Assert object JSON content matches (partial, supports @matchers)",
				Example: `"files" object "uploads/user.json" content should match:
  """
  {"id": "@number", "name": "John"}
  """`,
				Handler: r.objectContentShouldMatch,
			},
			{
				Group:       "Object Assertions",
				Pattern:     `^"{resource}" object "([^"]*)" size should be "(\d+)" bytes$`,
				Description: "Assert object size in bytes",
				Example:     `"files" object "uploads/hello.txt" size should be "11" bytes`,
				Handler:     r.objectSizeShouldBe,
			},
			{
				Group:       "Object Assertions",
				Pattern:     `^"{resource}" object "([^"]*)" content type should be "([^"]*)"$`,
				Description: "Assert object content type",
				Example:     `"files" object "uploads/a.csv" content type should be "text/csv"`,
				Handler:     r.objectContentTypeShouldBe,
			},
			{
				Group:       "Object Assertions",
				Pattern:     `^"{resource}" object "([^"]*)" metadata "([^"]*)" should be "([^"]*)"$`,
				Description: "Assert a user metadata value on an object",
				Example:     `"files" object "uploads/a.csv" metadata "owner" should be "billing"`,
				Handler:     r.objectMetadataShouldBe,
			},

			// Capture
			{
				Group:       "Capture",
				Pattern:     `^"{resource}" object "([^"]*)" content is captured as "([^"]*)"$`,
				Description: "Store object content in a variable for later steps",
				Example:     `"files" object "uploads/id.txt" content is captured as "user_id"`,
				Handler:     r.captureObjectContent,
			},
		},
	}
}

// ---------------------------------------------------------------------------
// helpers

// splitPath splits a "bucket/key" path into its two parts.
func splitPath(path string) (bucket, key string, err error) {
	path = strings.TrimPrefix(ReplaceVariables(path), "s3://")
	bucket, key, found := strings.Cut(path, "/")
	if !found || bucket == "" || key == "" {
		return "", "", fmt.Errorf("object path %q must be in the form \"bucket/key\"", path)
	}
	return bucket, key, nil
}

func isNotFound(err error) bool {
	var nsk *types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nsb *types.NoSuchBucket
	if errors.As(err, &nsb) {
		return true
	}
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	// HeadObject/HeadBucket report 404 as a bare API error with no typed shape.
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "NoSuchBucket", "404":
			return true
		}
	}
	return false
}

func (r *S3) ensureBucket(ctx context.Context, bucket string) error {
	_, err := r.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return nil
	}
	if !isNotFound(err) {
		return err
	}
	_, err = r.client.CreateBucket(ctx, &s3.CreateBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		// Tolerate a concurrent create.
		var owned *types.BucketAlreadyOwnedByYou
		var exists *types.BucketAlreadyExists
		if errors.As(err, &owned) || errors.As(err, &exists) {
			return nil
		}
		return err
	}
	return nil
}

// listKeys returns every key in a bucket matching prefix, following pagination.
func (r *S3) listKeys(ctx context.Context, bucket, prefix string) ([]string, error) {
	var keys []string
	var token *string
	for {
		out, err := r.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
			Bucket:            aws.String(bucket),
			Prefix:            aws.String(prefix),
			ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}
		for _, obj := range out.Contents {
			keys = append(keys, aws.ToString(obj.Key))
		}
		if !aws.ToBool(out.IsTruncated) {
			return keys, nil
		}
		token = out.NextContinuationToken
	}
}

func (r *S3) emptyBucketCtx(ctx context.Context, bucket string) error {
	keys, err := r.listKeys(ctx, bucket, "")
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}
	// DeleteObjects accepts at most 1000 keys per call.
	for start := 0; start < len(keys); start += 1000 {
		end := min(start+1000, len(keys))
		ids := make([]types.ObjectIdentifier, 0, end-start)
		for _, k := range keys[start:end] {
			ids = append(ids, types.ObjectIdentifier{Key: aws.String(k)})
		}
		out, err := r.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(bucket),
			Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)},
		})
		if err != nil {
			return err
		}
		if len(out.Errors) > 0 {
			return fmt.Errorf("deleting %s/%s: %s",
				bucket, aws.ToString(out.Errors[0].Key), aws.ToString(out.Errors[0].Message))
		}
	}
	return nil
}

// getObject fetches an object body and caches it for chained assertions.
func (r *S3) getObject(ctx context.Context, path string) ([]byte, error) {
	bucket, key, err := splitPath(path)
	if err != nil {
		return nil, err
	}
	out, err := r.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("object %q does not exist", path)
		}
		return nil, err
	}
	defer out.Body.Close()

	body, err := io.ReadAll(out.Body)
	if err != nil {
		return nil, fmt.Errorf("reading object %q: %w", path, err)
	}
	return body, nil
}

func (r *S3) put(ctx context.Context, path string, body []byte, contentType string) error {
	bucket, key, err := splitPath(path)
	if err != nil {
		return err
	}
	if err := r.ensureBucket(ctx, bucket); err != nil {
		return fmt.Errorf("ensuring bucket %q: %w", bucket, err)
	}
	in := &s3.PutObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(body),
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	_, err = r.client.PutObject(ctx, in)
	return err
}

// poll retries check until it passes or the timeout expires, returning the
// last failure so the user sees why it never became true.
func poll(timeout string, check func() error) error {
	d, err := time.ParseDuration(timeout)
	if err != nil {
		return fmt.Errorf("invalid timeout %q: %w", timeout, err)
	}
	deadline := time.Now().Add(d)
	lastErr := check()
	for lastErr != nil && time.Now().Before(deadline) {
		time.Sleep(100 * time.Millisecond)
		lastErr = check()
	}
	if lastErr != nil {
		return fmt.Errorf("not satisfied within %s: %w", timeout, lastErr)
	}
	return nil
}

// ---------------------------------------------------------------------------
// bucket setup

func (r *S3) bucketExists(bucket string) error {
	return r.ensureBucket(context.Background(), ReplaceVariables(bucket))
}

func (r *S3) emptyBucket(bucket string) error {
	return r.emptyBucketCtx(context.Background(), ReplaceVariables(bucket))
}

func (r *S3) deleteBucket(bucket string) error {
	ctx := context.Background()
	bucket = ReplaceVariables(bucket)
	if err := r.emptyBucketCtx(ctx, bucket); err != nil {
		return err
	}
	_, err := r.client.DeleteBucket(ctx, &s3.DeleteBucketInput{Bucket: aws.String(bucket)})
	if err != nil && isNotFound(err) {
		return nil
	}
	return err
}

// ---------------------------------------------------------------------------
// object setup

func (r *S3) putObject(path, content string) error {
	return r.put(context.Background(), path, []byte(ReplaceVariables(content)), "")
}

func (r *S3) putObjectDoc(path string, doc *godog.DocString) error {
	return r.put(context.Background(), path, []byte(ReplaceVariables(doc.Content)), "")
}

func (r *S3) putObjectWithContentType(path, content, contentType string) error {
	return r.put(context.Background(), path, []byte(ReplaceVariables(content)), contentType)
}

func (r *S3) putObjectFromFile(path, filePath string) error {
	data, err := os.ReadFile(ReplaceVariables(filePath))
	if err != nil {
		return fmt.Errorf("reading %q: %w", filePath, err)
	}
	return r.put(context.Background(), path, data, "")
}

func (r *S3) deleteObject(path string) error {
	bucket, key, err := splitPath(path)
	if err != nil {
		return err
	}
	_, err = r.client.DeleteObject(context.Background(), &s3.DeleteObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	return err
}

func (r *S3) putObjects(table *godog.Table) error {
	if len(table.Rows) < 2 {
		return errors.New("objects table needs a header row and at least one data row")
	}

	header := table.Rows[0].Cells
	pathCol, contentCol := -1, -1
	for i, cell := range header {
		switch strings.ToLower(strings.TrimSpace(cell.Value)) {
		case "path":
			pathCol = i
		case "content":
			contentCol = i
		}
	}
	if pathCol == -1 || contentCol == -1 {
		return errors.New("objects table needs \"path\" and \"content\" columns")
	}

	ctx := context.Background()
	for i, row := range table.Rows[1:] {
		if len(row.Cells) <= pathCol || len(row.Cells) <= contentCol {
			return fmt.Errorf("objects table row %d has %d columns, expected at least %d",
				i+1, len(row.Cells), max(pathCol, contentCol)+1)
		}
		path := row.Cells[pathCol].Value
		content := ReplaceVariables(row.Cells[contentCol].Value)
		if err := r.put(ctx, path, []byte(content), ""); err != nil {
			return fmt.Errorf("writing %q: %w", path, err)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// bucket assertions

func (r *S3) bucketShouldExist(bucket string) error {
	bucket = ReplaceVariables(bucket)
	_, err := r.client.HeadBucket(context.Background(), &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		if isNotFound(err) {
			return fmt.Errorf("bucket %q does not exist", bucket)
		}
		return err
	}
	return nil
}

func (r *S3) bucketShouldNotExist(bucket string) error {
	bucket = ReplaceVariables(bucket)
	_, err := r.client.HeadBucket(context.Background(), &s3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err == nil {
		return fmt.Errorf("bucket %q exists", bucket)
	}
	if isNotFound(err) {
		return nil
	}
	return err
}

func (r *S3) countObjects(bucket, prefix string) (int, error) {
	keys, err := r.listKeys(context.Background(), ReplaceVariables(bucket), prefix)
	if err != nil {
		if isNotFound(err) {
			return 0, fmt.Errorf("bucket %q does not exist", bucket)
		}
		return 0, err
	}
	return len(keys), nil
}

func (r *S3) bucketShouldHaveCount(bucket string, expected int) error {
	actual, err := r.countObjects(bucket, "")
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("bucket %q has %d objects, expected %d", bucket, actual, expected)
	}
	return nil
}

func (r *S3) bucketShouldBeEmpty(bucket string) error {
	actual, err := r.countObjects(bucket, "")
	if err != nil {
		return err
	}
	if actual != 0 {
		return fmt.Errorf("bucket %q has %d objects, expected it to be empty", bucket, actual)
	}
	return nil
}

func (r *S3) bucketShouldHaveCountWithPrefix(bucket string, expected int, prefix string) error {
	prefix = ReplaceVariables(prefix)
	actual, err := r.countObjects(bucket, prefix)
	if err != nil {
		return err
	}
	if actual != expected {
		return fmt.Errorf("bucket %q has %d objects with prefix %q, expected %d", bucket, actual, prefix, expected)
	}
	return nil
}

func (r *S3) bucketShouldHaveCountWithin(bucket string, expected int, timeout string) error {
	return poll(timeout, func() error { return r.bucketShouldHaveCount(bucket, expected) })
}

// ---------------------------------------------------------------------------
// object assertions

func (r *S3) headObject(path string) (*s3.HeadObjectOutput, error) {
	bucket, key, err := splitPath(path)
	if err != nil {
		return nil, err
	}
	out, err := r.client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, fmt.Errorf("object %q does not exist", path)
		}
		return nil, err
	}
	return out, nil
}

func (r *S3) objectShouldExist(path string) error {
	_, err := r.headObject(path)
	return err
}

func (r *S3) objectShouldNotExist(path string) error {
	bucket, key, err := splitPath(path)
	if err != nil {
		return err
	}
	_, err = r.client.HeadObject(context.Background(), &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err == nil {
		return fmt.Errorf("object %q exists", path)
	}
	if isNotFound(err) {
		return nil
	}
	return err
}

func (r *S3) objectShouldExistWithin(path, timeout string) error {
	return poll(timeout, func() error { return r.objectShouldExist(path) })
}

func (r *S3) objectContentShouldBe(path, expected string) error {
	body, err := r.getObject(context.Background(), path)
	if err != nil {
		return err
	}
	expected = ReplaceVariables(expected)
	if string(body) != expected {
		return fmt.Errorf("object %q content is %q, expected %q", path, string(body), expected)
	}
	return nil
}

func (r *S3) objectContentShouldBeDoc(path string, doc *godog.DocString) error {
	return r.objectContentShouldBe(path, strings.TrimSpace(doc.Content))
}

func (r *S3) objectContentShouldContain(path, substr string) error {
	body, err := r.getObject(context.Background(), path)
	if err != nil {
		return err
	}
	substr = ReplaceVariables(substr)
	if !strings.Contains(string(body), substr) {
		return fmt.Errorf("object %q content does not contain %q, got %q", path, substr, string(body))
	}
	return nil
}

func (r *S3) objectContentShouldMatch(path string, doc *godog.DocString) error {
	body, err := r.getObject(context.Background(), path)
	if err != nil {
		return err
	}

	var expected interface{}
	if err := json.Unmarshal([]byte(ReplaceVariables(doc.Content)), &expected); err != nil {
		return fmt.Errorf("parsing expected JSON: %w", err)
	}
	var actual interface{}
	if err := json.Unmarshal(body, &actual); err != nil {
		return fmt.Errorf("object %q is not valid JSON: %w", path, err)
	}
	return CompareJSON(expected, actual, "", true)
}

func (r *S3) objectSizeShouldBe(path string, expected int64) error {
	out, err := r.headObject(path)
	if err != nil {
		return err
	}
	actual := aws.ToInt64(out.ContentLength)
	if actual != expected {
		return fmt.Errorf("object %q is %d bytes, expected %d", path, actual, expected)
	}
	return nil
}

func (r *S3) objectContentTypeShouldBe(path, expected string) error {
	out, err := r.headObject(path)
	if err != nil {
		return err
	}
	actual := aws.ToString(out.ContentType)
	if actual != expected {
		return fmt.Errorf("object %q has content type %q, expected %q", path, actual, expected)
	}
	return nil
}

func (r *S3) objectMetadataShouldBe(path, key, expected string) error {
	out, err := r.headObject(path)
	if err != nil {
		return err
	}
	// S3 lowercases user metadata keys on the wire.
	actual, ok := out.Metadata[strings.ToLower(key)]
	if !ok {
		return fmt.Errorf("object %q has no metadata key %q", path, key)
	}
	expected = ReplaceVariables(expected)
	if actual != expected {
		return fmt.Errorf("object %q metadata %q is %q, expected %q", path, key, actual, expected)
	}
	return nil
}

// ---------------------------------------------------------------------------
// capture

func (r *S3) captureObjectContent(path, varName string) error {
	body, err := r.getObject(context.Background(), path)
	if err != nil {
		return err
	}
	SetVariable(varName, string(body))
	return nil
}

var _ Handler = (*S3)(nil)
