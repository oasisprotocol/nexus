package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSpecFileServer(t *testing.T) {
	// Create a temporary directory for test files.
	tmpDir, err := os.MkdirTemp("", "nexus-fileserver-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test YAML file.
	yamlContent := []byte("test: content")
	err = os.WriteFile(filepath.Join(tmpDir, "test.yaml"), yamlContent, 0o600)
	require.NoError(t, err)

	// Create a test text file.
	txtContent := []byte("hello world")
	err = os.WriteFile(filepath.Join(tmpDir, "test.txt"), txtContent, 0o600)
	require.NoError(t, err)

	// Create a subdirectory.
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0o750)
	require.NoError(t, err)

	// Create a symlink to the YAML file.
	err = os.Symlink(
		filepath.Join(tmpDir, "test.yaml"),
		filepath.Join(tmpDir, "symlink.yaml"),
	)
	require.NoError(t, err)

	srv := specFileServer{rootDir: tmpDir}

	tests := []struct {
		name         string
		path         string
		expectedCode int
		expectedType string
		expectedBody []byte
	}{
		{
			name:         "valid yaml file",
			path:         "/test.yaml",
			expectedCode: http.StatusOK,
			expectedType: "application/x-yaml",
			expectedBody: yamlContent,
		},
		{
			name:         "text file",
			path:         "/test.txt",
			expectedCode: http.StatusOK,
			expectedType: "text/plain; charset=utf-8",
			expectedBody: txtContent,
		},
		{
			name:         "non-existent file",
			path:         "/nonexistent.txt",
			expectedCode: http.StatusNotFound,
			expectedType: "text/plain; charset=utf-8",
			expectedBody: []byte("404 page not found\n"),
		},
		{
			name:         "very long path",
			path:         "/" + strings.Repeat("a", 65536) + ".yaml",
			expectedCode: http.StatusBadRequest,
			expectedType: "text/plain; charset=utf-8",
			expectedBody: []byte("Invalid path\n"),
		},
		{
			name:         "directory access attempt",
			path:         "/.",
			expectedCode: http.StatusForbidden,
			expectedType: "text/plain; charset=utf-8",
			expectedBody: []byte("403 Forbidden\n"),
		},
		{
			name:         "invalid utf8 path",
			path:         string([]byte{0xff, 0xfe, 0xfd}),
			expectedCode: http.StatusBadRequest,
			expectedType: "text/plain; charset=utf-8",
			expectedBody: []byte("Invalid path\n"),
		},
		{
			name:         "null byte in path",
			path:         "\x00test",
			expectedCode: http.StatusBadRequest,
			expectedType: "text/plain; charset=utf-8",
			expectedBody: []byte("Invalid path\n"),
		},
		{
			name:         "directory access",
			path:         "/subdir",
			expectedCode: http.StatusForbidden,
			expectedType: "text/plain; charset=utf-8",
			expectedBody: []byte("403 Forbidden\n"),
		},
		{
			name:         "symlink access",
			path:         "/symlink.yaml",
			expectedCode: http.StatusForbidden,
			expectedType: "text/plain; charset=utf-8",
			expectedBody: []byte("403 Forbidden\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &http.Request{
				Method: "GET",
				URL: &url.URL{
					Path: tt.path,
				},
			}
			w := httptest.NewRecorder()

			srv.ServeHTTP(w, req)

			require.Equal(t, tt.expectedCode, w.Code, fmt.Sprintf("%s: status code mismatch", tt.name))
			require.Equal(t, tt.expectedType, w.Header().Get("Content-Type"), fmt.Sprintf("%s: Content-Type mismatch", tt.name))

			if tt.expectedBody != nil {
				require.Equal(t, string(tt.expectedBody), w.Body.String(), fmt.Sprintf("%s: body mismatch", tt.name))
			}
		})
	}
}
