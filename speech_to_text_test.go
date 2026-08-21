package suzu

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGcpAuthCredentialsOption(t *testing.T) {
	t.Run("reads type from credential file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "creds.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"type":"authorized_user","refresh_token":"x"}`), 0o600))

		opt, err := gcpAuthCredentialsOption(path)
		require.NoError(t, err)
		assert.NotNil(t, opt)
	})

	t.Run("accepts service_account type", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "sa.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"type":"service_account","client_email":"a@b.c"}`), 0o600))

		opt, err := gcpAuthCredentialsOption(path)
		require.NoError(t, err)
		assert.NotNil(t, opt)
	})

	t.Run("accepts external_account type", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "ext.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"type":"external_account","audience":"//iam.googleapis.com/x"}`), 0o600))

		opt, err := gcpAuthCredentialsOption(path)
		require.NoError(t, err)
		assert.NotNil(t, opt)
	})

	t.Run("missing type field", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "no-type.json")
		require.NoError(t, os.WriteFile(path, []byte(`{"client_email":"a@b.c"}`), 0o600))

		opt, err := gcpAuthCredentialsOption(path)
		require.Error(t, err)
		assert.Nil(t, opt)
		assert.Contains(t, err.Error(), "missing `type` field")
	})

	t.Run("invalid json", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "broken.json")
		require.NoError(t, os.WriteFile(path, []byte(`{`), 0o600))

		opt, err := gcpAuthCredentialsOption(path)
		require.Error(t, err)
		assert.Nil(t, opt)
	})

	t.Run("file not found", func(t *testing.T) {
		opt, err := gcpAuthCredentialsOption(filepath.Join(t.TempDir(), "missing.json"))
		require.Error(t, err)
		assert.Nil(t, opt)
	})
}
