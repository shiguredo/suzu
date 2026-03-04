package suzu

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAmazonTranscribeClientV2_WithoutProfile_AppliesHTTPClientAndLogMode(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "dummy-access-key")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "dummy-secret-key")

	c := Config{
		Debug:                          true,
		AwsRegion:                      "ap-northeast-1",
		AwsHTTPDisableKeepAlives:       true,
		AwsHTTPIdleConnTimeoutSec:      90,
		AwsHTTPMaxIdleConns:            100,
		AwsHTTPMaxIdleConnsPerHost:     10,
		AwsHTTPMaxConnsPerHost:         20,
		AwsHTTPResponseHeaderTimeoutMs: 1234,
		AwsHTTPExpectContinueTimeoutMs: 567,
		AwsHTTPTLSHandshakeTimeoutMs:   8901,
	}

	client, err := NewAmazonTranscribeClientV2(c)
	require.NoError(t, err)

	opts := client.Options()
	require.NotNil(t, opts.HTTPClient)
	assert.NotEqual(t, 0, int(opts.ClientLogMode))

	httpClient, ok := opts.HTTPClient.(*http.Client)
	require.True(t, ok)

	tr, ok := httpClient.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, c.AwsHTTPDisableKeepAlives, tr.DisableKeepAlives)
	assert.Equal(t, time.Duration(c.AwsHTTPIdleConnTimeoutSec)*time.Second, tr.IdleConnTimeout)
	assert.Equal(t, c.AwsHTTPMaxIdleConns, tr.MaxIdleConns)
	assert.Equal(t, c.AwsHTTPMaxIdleConnsPerHost, tr.MaxIdleConnsPerHost)
	assert.Equal(t, c.AwsHTTPMaxConnsPerHost, tr.MaxConnsPerHost)
	assert.Equal(t, time.Duration(c.AwsHTTPResponseHeaderTimeoutMs)*time.Millisecond, tr.ResponseHeaderTimeout)
	assert.Equal(t, time.Duration(c.AwsHTTPExpectContinueTimeoutMs)*time.Millisecond, tr.ExpectContinueTimeout)
	assert.Equal(t, time.Duration(c.AwsHTTPTLSHandshakeTimeoutMs)*time.Millisecond, tr.TLSHandshakeTimeout)
}

func TestNewAmazonTranscribeClientV2_WithProfile_UsesCredentialFile(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "")
	t.Setenv("HOME", t.TempDir())

	credentialFilePath := filepath.Join(t.TempDir(), "credentials")
	credentialFile := `[profile-a]
aws_access_key_id = profile-a-access-key
aws_secret_access_key = profile-a-secret-key
`
	err := os.WriteFile(credentialFilePath, []byte(credentialFile), 0o600)
	require.NoError(t, err)

	c := Config{
		AwsRegion:         "ap-northeast-1",
		AwsProfile:        "profile-a",
		AwsCredentialFile: credentialFilePath,
	}

	client, err := NewAmazonTranscribeClientV2(c)
	require.NoError(t, err)

	creds, err := client.Options().Credentials.Retrieve(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "profile-a-access-key", creds.AccessKeyID)
	assert.Equal(t, "profile-a-secret-key", creds.SecretAccessKey)
}
