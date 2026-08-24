package uploadstorage

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	osscred "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
	"github.com/google/uuid"
)

type ossStore struct {
	client               *oss.Client
	bucket               string
	prefix               string
	serverSideEncryption string
}

func NewOSSStore(cfg Config) (Store, error) {
	if cfg.OSSBucket == "" {
		return nil, fmt.Errorf("UPLOAD_OSS_BUCKET is required")
	}
	var provider osscred.CredentialsProvider
	switch cfg.OSSCredentialType {
	case "ecs_ram_role":
		if cfg.OSSECSRAMRole != "" {
			provider = osscred.NewEcsRoleCredentialsProvider(osscred.EcsRamRole(cfg.OSSECSRAMRole))
		} else {
			provider = osscred.NewEcsRoleCredentialsProvider()
		}
	case "environment":
		provider = osscred.NewEnvironmentVariableCredentialsProvider()
	case "static":
		provider = osscred.NewStaticCredentialsProvider(cfg.OSSAccessKeyID, cfg.OSSAccessKeySecret, cfg.OSSSecurityToken)
	default:
		return nil, fmt.Errorf("unsupported upload OSS credential type %q", cfg.OSSCredentialType)
	}
	ossCfg := oss.LoadDefaultConfig().
		WithRegion(cfg.OSSRegion).
		WithCredentialsProvider(provider).
		WithRetryMaxAttempts(4)
	if cfg.OSSEndpoint != "" {
		ossCfg.WithEndpoint(cfg.OSSEndpoint)
	} else if cfg.OSSUseInternal {
		ossCfg.WithUseInternalEndpoint(true)
	}
	return &ossStore{
		client:               oss.NewClient(ossCfg),
		bucket:               cfg.OSSBucket,
		prefix:               strings.Trim(cfg.OSSPrefix, "/"),
		serverSideEncryption: cfg.OSSServerEncryption,
	}, nil
}

func (s *ossStore) Backend() string { return "oss" }

func (s *ossStore) Validate(ctx context.Context) error {
	versioning, err := s.client.GetBucketVersioning(ctx, &oss.GetBucketVersioningRequest{Bucket: oss.Ptr(s.bucket)})
	if err != nil {
		return fmt.Errorf("read OSS bucket versioning: %w", err)
	}
	if versioning.VersionStatus != nil && strings.TrimSpace(*versioning.VersionStatus) != "" {
		return fmt.Errorf("OSS upload bucket versioning must be disabled, got %s", *versioning.VersionStatus)
	}
	acl, err := s.client.GetBucketAcl(ctx, &oss.GetBucketAclRequest{Bucket: oss.Ptr(s.bucket)})
	if err != nil {
		return fmt.Errorf("read OSS bucket ACL: %w", err)
	}
	if acl.ACL == nil || *acl.ACL != "private" {
		got := "unknown"
		if acl.ACL != nil {
			got = *acl.ACL
		}
		return fmt.Errorf("OSS upload bucket ACL must be private, got %s", got)
	}
	policyStatus, err := s.client.GetBucketPolicyStatus(ctx, &oss.GetBucketPolicyStatusRequest{Bucket: oss.Ptr(s.bucket)})
	if err != nil {
		return fmt.Errorf("read OSS bucket policy status: %w", err)
	}
	if policyStatus.PolicyStatus != nil && policyStatus.PolicyStatus.IsPublic != nil && *policyStatus.PolicyStatus.IsPublic {
		return fmt.Errorf("OSS upload bucket policy allows public access")
	}
	return nil
}

func (s *ossStore) physicalKey(key string) (string, error) {
	if err := validateStorageKey(key); err != nil {
		return "", err
	}
	if s.prefix == "" {
		return key, nil
	}
	return s.prefix + "/" + key, nil
}

func (s *ossStore) Put(ctx context.Context, key string, body io.Reader, size int64, contentType string) error {
	physicalKey, err := s.physicalKey(key)
	if err != nil {
		return err
	}
	request := &oss.PutObjectRequest{
		Bucket:          oss.Ptr(s.bucket),
		Key:             oss.Ptr(physicalKey),
		Body:            body,
		ContentLength:   oss.Ptr(size),
		ForbidOverwrite: oss.Ptr("true"),
	}
	if contentType != "" {
		request.ContentType = oss.Ptr(contentType)
	}
	if s.serverSideEncryption != "" {
		request.ServerSideEncryption = oss.Ptr(s.serverSideEncryption)
	}
	_, err = s.client.PutObject(ctx, request)
	return err
}

func (s *ossStore) Open(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	physicalKey, err := s.physicalKey(key)
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	result, err := s.client.GetObject(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(physicalKey)})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	info := ObjectInfo{Key: key, Size: result.ContentLength}
	if result.LastModified != nil {
		info.LastModified = result.LastModified.UTC()
	}
	return result.Body, info, nil
}

func (s *ossStore) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	physicalKey, err := s.physicalKey(key)
	if err != nil {
		return ObjectInfo{}, err
	}
	result, err := s.client.HeadObject(ctx, &oss.HeadObjectRequest{Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(physicalKey)})
	if err != nil {
		return ObjectInfo{}, err
	}
	info := ObjectInfo{Key: key, Size: result.ContentLength}
	if result.LastModified != nil {
		info.LastModified = result.LastModified.UTC()
	}
	return info, nil
}

func (s *ossStore) Exists(ctx context.Context, key string) (bool, error) {
	physicalKey, err := s.physicalKey(key)
	if err != nil {
		return false, err
	}
	return s.client.IsObjectExist(ctx, s.bucket, physicalKey)
}

func (s *ossStore) Delete(ctx context.Context, key string) error {
	physicalKey, err := s.physicalKey(key)
	if err != nil {
		return err
	}
	_, err = s.client.DeleteObject(ctx, &oss.DeleteObjectRequest{Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(physicalKey)})
	return err
}

func (s *ossStore) EraseUserObjects(ctx context.Context, userID uuid.UUID) error {
	physicalPrefix, err := s.physicalKey(userPrefix(userID) + "placeholder")
	if err != nil {
		return err
	}
	physicalPrefix = strings.TrimSuffix(physicalPrefix, "placeholder")
	var keys []string
	var token *string
	for {
		result, err := s.client.ListObjectsV2(ctx, &oss.ListObjectsV2Request{
			Bucket: oss.Ptr(s.bucket), Prefix: oss.Ptr(physicalPrefix), MaxKeys: 1000, ContinuationToken: token,
		})
		if err != nil {
			return fmt.Errorf("list user upload objects: %w", err)
		}
		for _, object := range result.Contents {
			if object.Key != nil {
				keys = append(keys, *object.Key)
			}
		}
		if !result.IsTruncated || result.NextContinuationToken == nil || *result.NextContinuationToken == "" {
			break
		}
		token = result.NextContinuationToken
	}
	for _, key := range keys {
		if _, err := s.client.DeleteObject(ctx, &oss.DeleteObjectRequest{Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(key)}); err != nil {
			return fmt.Errorf("delete user upload object %s: %w", key, err)
		}
	}
	return nil
}

var _ Store = (*ossStore)(nil)
