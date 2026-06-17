package blob

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Store is an S3-compatible Store (MinIO locally, DO Spaces in production).
type S3Store struct {
	client *s3.Client
	bucket string
}

// S3Options configures an S3Store.
type S3Options struct {
	Endpoint     string // empty → AWS default endpoint resolution
	Region       string
	Bucket       string
	AccessKey    string
	SecretKey    string
	UsePathStyle bool // true for MinIO
}

// NewS3Store builds an S3-compatible store.
func NewS3Store(ctx context.Context, o S3Options) (*S3Store, error) {
	loadOpts := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(o.Region),
	}
	if o.AccessKey != "" {
		loadOpts = append(loadOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(o.AccessKey, o.SecretKey, ""),
		))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("loading aws config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(p *s3.Options) {
		if o.Endpoint != "" {
			p.BaseEndpoint = aws.String(o.Endpoint)
		}
		p.UsePathStyle = o.UsePathStyle
	})
	return &S3Store{client: client, bucket: o.Bucket}, nil
}

func (s *S3Store) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	// The S3 PutObject API needs a seekable body for signing; buffer when the
	// reader is not already a ReadSeeker. Artifacts are modestly sized tarballs.
	body, ok := r.(io.ReadSeeker)
	if !ok {
		buf, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		body = bytes.NewReader(buf)
		size = int64(len(buf))
	}
	in := &s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   body,
	}
	if contentType != "" {
		in.ContentType = aws.String(contentType)
	}
	if size >= 0 {
		in.ContentLength = aws.Int64(size)
	}
	_, err := s.client.PutObject(ctx, in)
	return err
}

func (s *S3Store) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nsk *types.NoSuchKey
		if errors.As(err, &nsk) {
			return nil, fmt.Errorf("blob %q: %w", key, os.ErrNotExist)
		}
		return nil, err
	}
	return out.Body, nil
}

func (s *S3Store) Exists(ctx context.Context, key string) (bool, error) {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var nf *types.NotFound
		if errors.As(err, &nf) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *S3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	return err
}

func (s *S3Store) Copy(ctx context.Context, src, dst string) error {
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(s.bucket + "/" + src),
		Key:        aws.String(dst),
	})
	return err
}
