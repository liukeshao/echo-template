package httpkit

import (
	"bytes"
	"io"
	"io/fs"
	"mime"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/liukeshao/echo-template/pkg/httperr"
)

// Static 将本地目录 root 挂载到指定 URL 前缀（等价于 StaticFS(prefix, os.DirFS(root))）。
func (r *Router) Static(prefix, root string) {
	r.StaticFS(prefix, os.DirFS(root))
}

// StaticFS 将 fs.FS 挂载到指定 URL 前缀（类似 Echo 的 StaticFS），自动去前缀并禁用目录浏览；
// 文件不存在或命中目录时通过 Router 的 ErrorEncoder 返回 RFC 9457 404（仅响应 GET / HEAD）。
func (r *Router) StaticFS(prefix string, fsys fs.FS) {
	cleanPrefix := "/" + strings.Trim(path.Clean("/"+prefix), "/")
	muxPattern := "GET /"
	stripPrefix := "/"
	if cleanPrefix != "/" {
		muxPattern = "GET " + cleanPrefix + "/"
		stripPrefix = cleanPrefix + "/"
	}

	r.mux.HandleFunc(muxPattern, func(w http.ResponseWriter, req *http.Request) {
		name, ok := strings.CutPrefix(path.Clean(req.URL.Path), stripPrefix)
		if !ok {
			writeStaticNotFound(w, req, r.errorEncoder)
			return
		}
		serveFileFS(w, req, fsys, name, r.errorEncoder)
	})
}

// File 将单个本地文件挂载到指定路径（类似 Echo 的 File，仅响应 GET / HEAD）。
func (r *Router) File(urlPath, filePath string) {
	clean := filepath.Clean(filePath)
	r.FileFS(urlPath, filepath.Base(clean), os.DirFS(filepath.Dir(clean)))
}

// FileFS 将 fs.FS 中的单个文件挂载到指定路径（类似 Echo 的 FileFS，仅响应 GET / HEAD）；
// 文件不存在或为目录时通过 Router 的 ErrorEncoder 返回 RFC 9457 404。
func (r *Router) FileFS(urlPath, file string, fsys fs.FS) {
	cleanPath := "/" + strings.TrimLeft(urlPath, "/")
	muxPattern := "GET " + cleanPath
	if strings.HasSuffix(cleanPath, "/") {
		// Go 1.22+ ServeMux 中以 / 结尾的模式为子树前缀匹配，加 {$} 保证单文件仅精确匹配该路径。
		muxPattern += "{$}"
	}
	name := strings.TrimPrefix(file, "/")

	r.mux.HandleFunc(muxPattern, func(w http.ResponseWriter, req *http.Request) {
		serveFileFS(w, req, fsys, name, r.errorEncoder)
	})
}

func serveFileFS(w http.ResponseWriter, req *http.Request, fsys fs.FS, name string, encodeErr httperr.Encoder) {
	if fsys == nil || !fs.ValidPath(name) {
		writeStaticNotFound(w, req, encodeErr)
		return
	}

	f, err := fsys.Open(name)
	if err != nil {
		writeStaticNotFound(w, req, encodeErr)
		return
	}
	defer func() { _ = f.Close() }()

	info, err := f.Stat()
	if err != nil || info.IsDir() {
		writeStaticNotFound(w, req, encodeErr)
		return
	}

	if w.Header().Get("Content-Type") == "" {
		if ct := contentTypeByExt(path.Ext(name)); ct != "" {
			w.Header().Set("Content-Type", ct)
		}
	}

	if rs, ok := f.(io.ReadSeeker); ok {
		http.ServeContent(w, req, info.Name(), info.ModTime(), rs)
		return
	}

	data, err := io.ReadAll(f)
	if err != nil {
		if encodeErr == nil {
			encodeErr = httperr.DefaultEncoder
		}
		encodeErr(w, req, httperr.Internal)
		return
	}
	http.ServeContent(w, req, info.Name(), info.ModTime(), bytes.NewReader(data))
}

func writeStaticNotFound(w http.ResponseWriter, req *http.Request, encodeErr httperr.Encoder) {
	if encodeErr == nil {
		encodeErr = httperr.DefaultEncoder
	}
	encodeErr(w, req, httperr.Problem{
		Status: http.StatusNotFound,
		Metadata: httperr.ResourceInfo{
			ResourceType: "static_asset",
			ResourceName: req.URL.Path,
		},
	})
}

func contentTypeByExt(ext string) string {
	if ct := mime.TypeByExtension(ext); ct != "" {
		return ct
	}
	switch strings.ToLower(ext) {
	case ".yaml", ".yml":
		return "application/yaml"
	default:
		return ""
	}
}
