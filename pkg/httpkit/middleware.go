package httpkit

import (
	"fmt"
	"log/slog"
	"math"
	"net/http"
	"runtime/debug"
	"time"
	"uuid"

	"github.com/liukeshao/go-rest-template/pkg/httperr"
	"github.com/liukeshao/go-rest-template/pkg/logging"
)

const (
	HeaderRequestID = "X-Request-ID"
	maxRequestIDLen = 128
)

// RequestID 读取或生成 UUID v7 格式的 X-Request-ID，写入响应头与 context。
// 客户端传入的值须满足长度 <= 128 且仅含 [a-zA-Z0-9._:-]，否则丢弃并生成新 UUID v7 以防日志污染。
func RequestID() Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rid := r.Header.Get(HeaderRequestID)
			if !isValidRequestID(rid) {
				rid = uuid.NewV7().String()
			}
			w.Header().Set(HeaderRequestID, rid)
			next.ServeHTTP(w, r.WithContext(logging.WithRequestID(r.Context(), rid)))
		})
	}
}

func isValidRequestID(s string) bool {
	if len(s) == 0 || len(s) > maxRequestIDLen {
		return false
	}
	for i := range len(s) {
		c := s[i]
		isAlphaNum := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
		if !isAlphaNum && c != '-' && c != '_' && c != '.' && c != ':' {
			return false
		}
	}
	return true
}

func BodyLimit(maxBytes int64) Middleware {
	if maxBytes <= 0 {
		return func(next http.Handler) http.Handler { return next }
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// Recover 捕获业务 panic 并转为 RFC 9457 500 响应；按标准库契约放行 http.ErrAbortHandler，
// 且在响应头已提交时不重复写出错误响应体。
func Recover(enc httperr.Encoder) Middleware {
	if enc == nil {
		enc = httperr.DefaultEncoder
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tw := &headerTrackWriter{ResponseWriter: w}
			defer func() {
				if rec := recover(); rec != nil {
					if rec == http.ErrAbortHandler {
						panic(rec)
					}
					slog.ErrorContext(r.Context(), "panic recovered",
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					if !tw.wroteHeader {
						enc(tw, r, fmt.Errorf("panic: %v", rec))
					}
				}
			}()
			next.ServeHTTP(tw, r)
		})
	}
}

// AccessLog 记录结构化请求访问日志（毫秒耗时），可通过 skipPaths 忽略探针等高频路径（如 /healthz、/readyz）。
func AccessLog(skipPaths ...string) Middleware {
	var skip map[string]struct{}
	if len(skipPaths) > 0 {
		skip = make(map[string]struct{}, len(skipPaths))
		for _, p := range skipPaths {
			if p != "" {
				skip[p] = struct{}{}
			}
		}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := skip[r.URL.Path]; ok {
				next.ServeHTTP(w, r)
				return
			}
			start := time.Now()
			sw := &statusCaptureWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			latencyMS := math.Round(float64(time.Since(start).Microseconds())/10) / 100
			slog.InfoContext(r.Context(), "http request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"latency_ms", latencyMS,
			)
		})
	}
}

type headerTrackWriter struct {
	http.ResponseWriter
	wroteHeader bool
}

func (w *headerTrackWriter) WriteHeader(code int) {
	w.wroteHeader = true
	w.ResponseWriter.WriteHeader(code)
}

func (w *headerTrackWriter) Write(b []byte) (int, error) {
	w.wroteHeader = true
	return w.ResponseWriter.Write(b)
}

func (w *headerTrackWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

type statusCaptureWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (w *statusCaptureWriter) WriteHeader(code int) {
	if !w.wroteHeader {
		w.status = code
		w.wroteHeader = true
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusCaptureWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader {
		w.wroteHeader = true
	}
	return w.ResponseWriter.Write(b)
}

// Unwrap 暴露底层 http.ResponseWriter，兼容 Go 1.20+ http.ResponseController（Flusher / Hijacker 等）。
func (w *statusCaptureWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}
