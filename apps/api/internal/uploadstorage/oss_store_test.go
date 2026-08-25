package uploadstorage

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss"
	osscred "github.com/aliyun/alibabacloud-oss-go-sdk-v2/oss/credentials"
)

func testOSSStore(t *testing.T, versionStatus, acl string, policyPublic bool) *ossStore {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/xml")
		switch {
		case r.URL.Query().Has("versioning"):
			if versionStatus == "" {
				fmt.Fprint(w, `<VersioningConfiguration></VersioningConfiguration>`)
			} else {
				fmt.Fprintf(w, `<VersioningConfiguration><Status>%s</Status></VersioningConfiguration>`, versionStatus)
			}
		case r.URL.Query().Has("acl"):
			fmt.Fprintf(w, `<AccessControlPolicy><AccessControlList><Grant>%s</Grant></AccessControlList></AccessControlPolicy>`, acl)
		case r.URL.Query().Has("policyStatus"):
			fmt.Fprintf(w, `<PolicyStatus><IsPublic>%t</IsPublic></PolicyStatus>`, policyPublic)
		default:
			http.Error(w, "unexpected OSS operation", http.StatusBadRequest)
		}
	}))
	t.Cleanup(server.Close)
	cfg := oss.LoadDefaultConfig().
		WithRegion("cn-hangzhou").
		WithEndpoint(server.URL).
		WithUsePathStyle(true).
		WithCredentialsProvider(osscred.NewStaticCredentialsProvider("test-id", "test-secret")).
		WithRetryMaxAttempts(1)
	return &ossStore{client: oss.NewClient(cfg), bucket: "private-bucket", prefix: "bodysense/production/uploads"}
}

func TestOSSStoreValidateAcceptsPrivateUnversionedBucket(t *testing.T) {
	if err := testOSSStore(t, "", "private", false).Validate(context.Background()); err != nil {
		t.Fatalf("safe OSS bucket rejected: %v", err)
	}
}

func TestOSSStoreValidateRejectsVersionedOrPublicBucket(t *testing.T) {
	cases := []struct {
		name          string
		versionStatus string
		acl           string
		policyPublic  bool
		want          string
	}{
		{name: "versioning enabled", versionStatus: "Enabled", acl: "private", want: "versioning must be disabled"},
		{name: "versioning suspended", versionStatus: "Suspended", acl: "private", want: "versioning must be disabled"},
		{name: "public acl", acl: "public-read", want: "ACL must be private"},
		{name: "public policy", acl: "private", policyPublic: true, want: "policy allows public access"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := testOSSStore(t, tc.versionStatus, tc.acl, tc.policyPublic).Validate(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate error=%v want contains %q", err, tc.want)
			}
		})
	}
}
