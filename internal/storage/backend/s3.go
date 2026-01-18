package backend

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3Config holds configuration for the S3 backend.
type S3Config struct {
	// Bucket is the S3 bucket name (required)
	Bucket string

	// Region is the AWS region (e.g., "us-east-1")
	Region string

	// Endpoint is the S3 endpoint URL (optional, for S3-compatible services)
	// Examples: "https://s3.amazonaws.com", "http://localhost:9000" (MinIO)
	Endpoint string

	// AccessKeyID is the AWS access key (optional if using IAM roles)
	AccessKeyID string

	// SecretAccessKey is the AWS secret key (optional if using IAM roles)
	SecretAccessKey string

	// UsePathStyle forces path-style addressing (required for MinIO and some S3-compatible services)
	UsePathStyle bool

	// Prefix is an optional prefix for all keys (e.g., "storage/")
	Prefix string
}

// S3Backend implements Backend using Amazon S3 or S3-compatible storage.
type S3Backend struct {
	client *s3.Client
	bucket string
	prefix string
}

// NewS3 creates a new S3 backend.
func NewS3(ctx context.Context, cfg S3Config) (*S3Backend, error) {
	if cfg.Bucket == "" {
		return nil, &Error{Op: "NewS3", Err: fmt.Errorf("bucket name is required")}
	}

	// Build AWS config options
	var opts []func(*config.LoadOptions) error

	if cfg.Region != "" {
		opts = append(opts, config.WithRegion(cfg.Region))
	}

	// Use static credentials if provided, otherwise use default credential chain
	// (environment variables, IAM roles, etc.)
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		opts = append(opts, config.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		))
	}

	awsCfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, &Error{Op: "NewS3", Err: fmt.Errorf("load AWS config: %w", err)}
	}

	// Build S3 client options
	var s3Opts []func(*s3.Options)

	if cfg.Endpoint != "" {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		})
	}

	if cfg.UsePathStyle {
		s3Opts = append(s3Opts, func(o *s3.Options) {
			o.UsePathStyle = true
		})
	}

	client := s3.NewFromConfig(awsCfg, s3Opts...)

	// Verify bucket exists and is accessible
	_, err = client.HeadBucket(ctx, &s3.HeadBucketInput{
		Bucket: aws.String(cfg.Bucket),
	})
	if err != nil {
		return nil, &Error{Op: "NewS3", Err: fmt.Errorf("bucket not accessible: %w", err)}
	}

	return &S3Backend{
		client: client,
		bucket: cfg.Bucket,
		prefix: cfg.Prefix,
	}, nil
}

// fullKey returns the full S3 key including prefix.
func (b *S3Backend) fullKey(key string) string {
	if b.prefix == "" {
		return key
	}
	return b.prefix + key
}

// stripPrefix removes the prefix from an S3 key.
func (b *S3Backend) stripPrefix(key string) string {
	if b.prefix == "" {
		return key
	}
	return strings.TrimPrefix(key, b.prefix)
}

// Exists checks if a file exists at the given key.
func (b *S3Backend) Exists(ctx context.Context, key string) (bool, error) {
	_, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.fullKey(key)),
	})
	if err != nil {
		// Check if it's a "not found" error
		var nsk *types.NotFound
		if ok := errors.As(err, &nsk); ok {
			return false, nil
		}
		// Also check for NoSuchKey
		var noKey *types.NoSuchKey
		if ok := errors.As(err, &noKey); ok {
			return false, nil
		}
		// Check error message for 404
		if strings.Contains(err.Error(), "NotFound") || strings.Contains(err.Error(), "404") {
			return false, nil
		}
		return false, &Error{Op: "Exists", Key: key, Err: err}
	}
	return true, nil
}

// Attributes returns metadata for a file.
func (b *S3Backend) Attributes(ctx context.Context, key string) (*FileInfo, error) {
	output, err := b.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.fullKey(key)),
	})
	if err != nil {
		if isNotFoundError(err) {
			return nil, &Error{Op: "Attributes", Key: key, Err: errNotFound{}}
		}
		return nil, &Error{Op: "Attributes", Key: key, Err: err}
	}

	var contentType string
	if output.ContentType != nil {
		contentType = *output.ContentType
	}

	var etag string
	if output.ETag != nil {
		// S3 ETags are quoted, remove quotes
		etag = strings.Trim(*output.ETag, "\"")
	}

	var modTime time.Time
	if output.LastModified != nil {
		modTime = *output.LastModified
	}

	var size int64
	if output.ContentLength != nil {
		size = *output.ContentLength
	}

	return &FileInfo{
		Key:         key,
		Size:        size,
		ContentType: contentType,
		ETag:        etag,
		ModTime:     modTime,
	}, nil
}

// Reader returns a reader for the file content.
func (b *S3Backend) Reader(ctx context.Context, key string) (io.ReadCloser, *FileInfo, error) {
	output, err := b.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.fullKey(key)),
	})
	if err != nil {
		if isNotFoundError(err) {
			return nil, nil, &Error{Op: "Reader", Key: key, Err: errNotFound{}}
		}
		return nil, nil, &Error{Op: "Reader", Key: key, Err: err}
	}

	var contentType string
	if output.ContentType != nil {
		contentType = *output.ContentType
	}

	var etag string
	if output.ETag != nil {
		etag = strings.Trim(*output.ETag, "\"")
	}

	var modTime time.Time
	if output.LastModified != nil {
		modTime = *output.LastModified
	}

	var size int64
	if output.ContentLength != nil {
		size = *output.ContentLength
	}

	info := &FileInfo{
		Key:         key,
		Size:        size,
		ContentType: contentType,
		ETag:        etag,
		ModTime:     modTime,
	}

	return output.Body, info, nil
}

// Write stores content at the given key.
func (b *S3Backend) Write(ctx context.Context, key string, content io.Reader, size int64, contentType string) (*FileInfo, error) {
	// For S3, we need to buffer the content to calculate MD5 and know the size
	// This is required for the Content-MD5 header
	var buf bytes.Buffer
	h := md5.New()
	writer := io.MultiWriter(&buf, h)

	var written int64
	var err error
	if size >= 0 {
		written, err = io.CopyN(writer, content, size)
	} else {
		written, err = io.Copy(writer, content)
	}
	if err != nil && err != io.EOF {
		return nil, &Error{Op: "Write", Key: key, Err: fmt.Errorf("buffer content: %w", err)}
	}

	etag := hex.EncodeToString(h.Sum(nil))

	input := &s3.PutObjectInput{
		Bucket:        aws.String(b.bucket),
		Key:           aws.String(b.fullKey(key)),
		Body:          bytes.NewReader(buf.Bytes()),
		ContentLength: aws.Int64(written),
	}

	if contentType != "" {
		input.ContentType = aws.String(contentType)
	}

	_, err = b.client.PutObject(ctx, input)
	if err != nil {
		return nil, &Error{Op: "Write", Key: key, Err: err}
	}

	return &FileInfo{
		Key:         key,
		Size:        written,
		ContentType: contentType,
		ETag:        etag,
		ModTime:     time.Now(),
	}, nil
}

// Delete removes a file at the given key.
func (b *S3Backend) Delete(ctx context.Context, key string) error {
	_, err := b.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(b.bucket),
		Key:    aws.String(b.fullKey(key)),
	})
	if err != nil {
		// S3 DeleteObject is idempotent, but check for other errors
		if !isNotFoundError(err) {
			return &Error{Op: "Delete", Key: key, Err: err}
		}
	}
	return nil
}

// DeletePrefix removes all files with the given prefix.
func (b *S3Backend) DeletePrefix(ctx context.Context, prefix string) []error {
	var errors []error
	fullPrefix := b.fullKey(prefix)

	// List all objects with the prefix
	paginator := s3.NewListObjectsV2Paginator(b.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(b.bucket),
		Prefix: aws.String(fullPrefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			errors = append(errors, &Error{Op: "DeletePrefix", Key: prefix, Err: err})
			return errors
		}

		if len(page.Contents) == 0 {
			continue
		}

		// Build delete request
		var objects []types.ObjectIdentifier
		for _, obj := range page.Contents {
			objects = append(objects, types.ObjectIdentifier{
				Key: obj.Key,
			})
		}

		// Delete objects in batch
		_, err = b.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
			Bucket: aws.String(b.bucket),
			Delete: &types.Delete{
				Objects: objects,
				Quiet:   aws.Bool(true),
			},
		})
		if err != nil {
			errors = append(errors, &Error{Op: "DeletePrefix", Key: prefix, Err: err})
		}
	}

	return errors
}

// List returns files with the given prefix.
func (b *S3Backend) List(ctx context.Context, prefix string, limit int, cursor string) ([]FileInfo, string, error) {
	if limit <= 0 {
		limit = 1000
	}

	input := &s3.ListObjectsV2Input{
		Bucket:  aws.String(b.bucket),
		Prefix:  aws.String(b.fullKey(prefix)),
		MaxKeys: aws.Int32(int32(limit)),
	}

	if cursor != "" {
		input.StartAfter = aws.String(b.fullKey(cursor))
	}

	output, err := b.client.ListObjectsV2(ctx, input)
	if err != nil {
		return nil, "", &Error{Op: "List", Key: prefix, Err: err}
	}

	var files []FileInfo
	for _, obj := range output.Contents {
		var key string
		if obj.Key != nil {
			key = b.stripPrefix(*obj.Key)
		}

		var size int64
		if obj.Size != nil {
			size = *obj.Size
		}

		var etag string
		if obj.ETag != nil {
			etag = strings.Trim(*obj.ETag, "\"")
		}

		var modTime time.Time
		if obj.LastModified != nil {
			modTime = *obj.LastModified
		}

		files = append(files, FileInfo{
			Key:     key,
			Size:    size,
			ETag:    etag,
			ModTime: modTime,
		})
	}

	var nextCursor string
	if output.IsTruncated != nil && *output.IsTruncated && len(files) > 0 {
		nextCursor = files[len(files)-1].Key
	}

	return files, nextCursor, nil
}

// Copy duplicates a file from srcKey to dstKey.
func (b *S3Backend) Copy(ctx context.Context, srcKey, dstKey string) error {
	// S3 CopyObject requires the source to be specified as bucket/key
	copySource := b.bucket + "/" + b.fullKey(srcKey)

	_, err := b.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(b.bucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(b.fullKey(dstKey)),
	})
	if err != nil {
		if isNotFoundError(err) {
			return &Error{Op: "Copy", Key: srcKey, Err: errNotFound{}}
		}
		return &Error{Op: "Copy", Key: srcKey, Err: err}
	}

	return nil
}

// Close releases resources.
func (b *S3Backend) Close() error {
	// S3 client doesn't need explicit cleanup
	return nil
}

// isNotFoundError checks if an error indicates the object was not found.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}

	var nsk *types.NotFound
	if ok := errors.As(err, &nsk); ok {
		return true
	}

	var noKey *types.NoSuchKey
	if ok := errors.As(err, &noKey); ok {
		return true
	}

	// Check error message as fallback
	errStr := err.Error()
	return strings.Contains(errStr, "NotFound") ||
		strings.Contains(errStr, "NoSuchKey") ||
		strings.Contains(errStr, "404")
}
