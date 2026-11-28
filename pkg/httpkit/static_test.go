package httpkit_test

import (
	"bytes"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/liukeshao/go-rest-template/pkg/httperr"
	"github.com/liukeshao/go-rest-template/pkg/httpkit"
)

func TestRouter_StaticFS(t *testing.T) {
	t.Parallel()

	assets := fstest.MapFS{
		"js/app.js":     {Data: []byte(`console.log("ok")`), ModTime: time.Unix(1700000000, 0)},
		"css/style.css": {Data: []byte(`body { color: #000; }`)},
		"spec.yaml":     {Data: []byte("openapi: 3.1.0\n")},
	}

	router := httpkit.NewRouter()
	router.StaticFS("/static", assets)
	router.StaticFS("/assets/", assets)
	router.StaticFS("/nilfs", nil)

	// Custom error encoder configured AFTER StaticFS must still be used at request time.
	var customCalled bool
	router.SetErrorEncoder(func(w http.ResponseWriter, r *http.Request, err error) {
		customCalled = true
		httperr.DefaultEncoder(w, r, err)
	})

	call := func(method, target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(method, target, nil))
		return rec
	}

	// Serves nested static files with Content-Type and supports HEAD.
	rec := call(http.MethodGet, "/static/js/app.js")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, `console.log("ok")`, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "javascript")

	rec = call(http.MethodHead, "/static/css/style.css")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/css")

	// Trailing slash in prefix registration behaves identically.
	rec = call(http.MethodGet, "/assets/spec.yaml")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/yaml", rec.Header().Get("Content-Type"))

	// Directory listing and missing files return RFC 9457 404 with ResourceInfo(static_asset).
	for _, p := range []string{"/static/", "/static/js/", "/static/missing.js", "/nilfs/a.txt"} {
		customCalled = false
		rec = call(http.MethodGet, p)
		assert.Equal(t, http.StatusNotFound, rec.Code, "path=%s", p)
		assert.True(t, customCalled, "path=%s should use router error encoder", p)
		prob := decodeProblem(t, rec)
		meta, err := httperr.DecodeMetadata[httperr.ResourceInfo](prob)
		require.NoError(t, err)
		assert.Equal(t, "static_asset", meta.ResourceType)
		assert.Equal(t, p, meta.ResourceName)
	}

	// Non-GET/HEAD method returns RFC 9457 405.
	rec = call(http.MethodPost, "/static/js/app.js")
	assert.Equal(t, http.StatusMethodNotAllowed, rec.Code)
	assert.Equal(t, http.StatusMethodNotAllowed, decodeProblem(t, rec).Status)
}

func TestRouter_FileFS(t *testing.T) {
	t.Parallel()

	docsFS := fstest.MapFS{
		"index.html":           {Data: []byte("<html>docs</html>")},
		"openapi/openapi.yaml": {Data: []byte("openapi: 3.1.0\n")},
		"sub/nested.txt":       {Data: []byte("hello")},
	}

	router := httpkit.NewRouter()
	router.FileFS("/", "index.html", docsFS)
	router.FileFS("/docs", "index.html", docsFS)
	router.FileFS("/index.html", "index.html", docsFS)
	router.FileFS("/openapi.yaml", "openapi/openapi.yaml", docsFS)
	router.FileFS("/missing", "does-not-exist.txt", docsFS)
	router.FileFS("/dir", "sub", docsFS)

	call := func(target string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, target, nil))
		return rec
	}

	// Exact root match serves index.html and does NOT swallow unmatched paths.
	rec := call("/")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "<html>docs</html>", rec.Body.String())
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")

	rec = call("/unmatched")
	assert.Equal(t, http.StatusNotFound, rec.Code)
	meta, err := httperr.DecodeMetadata[httperr.ResourceInfo](decodeProblem(t, rec))
	require.NoError(t, err)
	assert.Equal(t, "route", meta.ResourceType)

	// /index.html serves directly without http.ServeFileFS's 301 redirect quirk.
	rec = call("/index.html")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "<html>docs</html>", rec.Body.String())

	// /docs and /openapi.yaml serve with expected Content-Type.
	rec = call("/docs")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Contains(t, rec.Header().Get("Content-Type"), "text/html")

	rec = call("/openapi.yaml")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/yaml", rec.Header().Get("Content-Type"))

	// Missing file and directory return RFC 9457 404.
	for _, p := range []string{"/missing", "/dir"} {
		rec = call(p)
		assert.Equal(t, http.StatusNotFound, rec.Code, "path=%s", p)
		meta, err = httperr.DecodeMetadata[httperr.ResourceInfo](decodeProblem(t, rec))
		require.NoError(t, err)
		assert.Equal(t, "static_asset", meta.ResourceType)
	}
}

func TestRouter_StaticAndFileOnDisk(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "sub"), 0o755))
	singleFile := filepath.Join(dir, "robots.txt")
	require.NoError(t, os.WriteFile(singleFile, []byte("User-agent: *\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "sub", "hello.txt"), []byte("disk-ok"), 0o600))

	router := httpkit.NewRouter()
	router.Static("/files", dir)
	router.File("/robots.txt", singleFile)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/files/sub/hello.txt", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "disk-ok", rec.Body.String())

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/robots.txt", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "User-agent: *\n", rec.Body.String())
}

// nonSeekableFS wraps an fs.FS so opened files do not implement io.ReadSeeker.
type nonSeekableFS struct{ inner fs.FS }

type nonSeekableFile struct {
	info fs.FileInfo
	r    *bytes.Reader
}

func (f nonSeekableFS) Open(name string) (fs.File, error) {
	file, err := f.inner.Open(name)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	return &nonSeekableFile{info: info, r: bytes.NewReader(data)}, nil
}

func (f *nonSeekableFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *nonSeekableFile) Read(p []byte) (int, error) { return f.r.Read(p) }
func (f *nonSeekableFile) Close() error               { return nil }

func TestRouter_StaticFS_RootAndNonSeekable(t *testing.T) {
	t.Parallel()

	fsys := nonSeekableFS{
		inner: fstest.MapFS{
			"hello.txt": {Data: []byte("streamed")},
		},
	}

	router := httpkit.NewRouter()
	router.StaticFS("/", fsys)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/hello.txt", nil))
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "streamed", rec.Body.String())

	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusNotFound, rec.Code)
}
