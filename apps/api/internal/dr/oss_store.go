package dr

import (
	"context"
	"fmt"
	"io"
	"sort"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	osscred "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

type ossStore struct {
	client   *oss.Client
	uploader *oss.Uploader
	bucket   string
}

func NewOSSStore(cfg Config) (ObjectStore, error) {
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
		return nil, fmt.Errorf("unsupported OSS credential type %q", cfg.OSSCredentialType)
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
	client := oss.NewClient(ossCfg)
	return &ossStore{client: client, uploader: oss.NewUploader(client), bucket: cfg.OSSBucket}, nil
}

func (s *ossStore) Put(ctx context.Context, key string, body io.Reader, size int64, options PutOptions) error {
	request := &oss.PutObjectRequest{
		Bucket:        oss.Ptr(s.bucket),
		Key:           oss.Ptr(key),
		Body:          body,
		ContentLength: oss.Ptr(size),
		Metadata:      options.Metadata,
	}
	if options.ContentType != "" {
		request.ContentType = oss.Ptr(options.ContentType)
	}
	if options.ForbidOverwrite {
		request.ForbidOverwrite = oss.Ptr("true")
	}
	if options.ServerSideEncryption != "" {
		request.ServerSideEncryption = oss.Ptr(options.ServerSideEncryption)
	}
	_, err := s.client.PutObject(ctx, request)
	return err
}

func (s *ossStore) PutFile(ctx context.Context, key, path string, options PutOptions) error {
	request := &oss.PutObjectRequest{
		Bucket:   oss.Ptr(s.bucket),
		Key:      oss.Ptr(key),
		Metadata: options.Metadata,
	}
	if options.ContentType != "" {
		request.ContentType = oss.Ptr(options.ContentType)
	}
	if options.ForbidOverwrite {
		request.ForbidOverwrite = oss.Ptr("true")
	}
	if options.ServerSideEncryption != "" {
		request.ServerSideEncryption = oss.Ptr(options.ServerSideEncryption)
	}
	_, err := s.uploader.UploadFile(ctx, request, path)
	return err
}

func (s *ossStore) Get(ctx context.Context, key string) (io.ReadCloser, ObjectInfo, error) {
	result, err := s.client.GetObject(ctx, &oss.GetObjectRequest{Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(key)})
	if err != nil {
		return nil, ObjectInfo{}, err
	}
	info := ObjectInfo{Key: key, Size: result.ContentLength, Metadata: result.Metadata}
	if result.LastModified != nil {
		info.LastModified = result.LastModified.UTC()
	}
	return result.Body, info, nil
}

func (s *ossStore) Head(ctx context.Context, key string) (ObjectInfo, error) {
	result, err := s.client.HeadObject(ctx, &oss.HeadObjectRequest{Bucket: oss.Ptr(s.bucket), Key: oss.Ptr(key)})
	if err != nil {
		return ObjectInfo{}, err
	}
	info := ObjectInfo{Key: key, Size: result.ContentLength, Metadata: result.Metadata}
	if result.LastModified != nil {
		info.LastModified = result.LastModified.UTC()
	}
	return info, nil
}

func (s *ossStore) List(ctx context.Context, prefix string) ([]ObjectInfo, error) {
	var result []ObjectInfo
	var token *string
	for {
		response, err := s.client.ListObjectsV2(ctx, &oss.ListObjectsV2Request{
			Bucket: oss.Ptr(s.bucket), Prefix: oss.Ptr(prefix), MaxKeys: 1000, ContinuationToken: token,
		})
		if err != nil {
			return nil, err
		}
		for _, item := range response.Contents {
			if item.Key == nil {
				continue
			}
			info := ObjectInfo{Key: *item.Key, Size: item.Size}
			if item.LastModified != nil {
				info.LastModified = item.LastModified.UTC()
			}
			result = append(result, info)
		}
		if !response.IsTruncated || response.NextContinuationToken == nil || *response.NextContinuationToken == "" {
			break
		}
		token = response.NextContinuationToken
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Key < result[j].Key })
	return result, nil
}

var _ ObjectStore = (*ossStore)(nil)
