package s3

import (
	"errors"
	"fmt"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	s3svc "github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

type S3 interface {
	List() (interface{}, error)
	Download(folder, file, outputFile string) error
	Upload(target string, payload []byte) (int, error)
	Delete(target string) error
	Ready() error
	Close() error
}

type s3 struct {
	endpoint string
	region   string
}

// Open creates a new instance of S3
func Open(params map[string]string) (*s3, error) {
	endpoint, ok := params["endpoint"]
	if !ok {
		return nil, errors.New("filestore/s3: endpoint is required")
	}

	region, ok := params["region"]
	if !ok {
		return nil, errors.New("filestore/s3: region is required")
	}

	s := s3{
		endpoint: endpoint,
		region:   region,
	}

	return &s, nil
}

func (s *s3) List() (interface{}, error) {
	return nil, nil
}

// Download connects to the s3 instance and downloads
func (s *s3) Download(bucket, key, outputFile string) error {
	sess := session.Must(session.NewSession())
	svc := s3svc.New(sess, aws.NewConfig().WithEndpoint(s.endpoint).WithRegion(s.region).WithS3ForcePathStyle(true))
	downloader := s3manager.NewDownloaderWithClient(svc)

	// Create a file to write the S3 Object contents to.
	f, err := os.Create(outputFile)
	if err != nil {
		return fmt.Errorf("failed to create file %q, %v", outputFile, err)
	}

	// Write the contents of S3 Object to the file
	_, err = downloader.Download(f, &s3svc.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to download file, %v", err)
	}
	return nil
}

func (s *s3) Upload(target string, payload []byte) (int, error) {
	return 0, nil
}

func (s *s3) Delete(target string) error {
	return nil
}

func (s *s3) Ready() error {
	return nil
}

func (s *s3) Close() error {
	return nil
}
