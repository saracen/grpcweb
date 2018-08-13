package grpcweb

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"io"
	"net/http"
	"strings"
)

// gRPC content-types
const (
	ContentTypeGRPC             = "application/grpc"
	ContentTypeGRPCWeb          = "application/grpc-web"
	ContentTypeGRPCWebProto     = "application/grpc-web+proto"
	ContentTypeGRPCWebText      = "application/grpc-web-text"
	ContentTypeGRPCWebTextProto = "application/grpc-web-text+proto"
)

const (
	headerContentType        = "content-type"
	headerContentLength      = "content-length"
	headerTE                 = "te"
	headerGRPCAcceptEncoding = "grpc-accept-encoding"
	headerAccept             = "accept"
	headerTrailer            = "trailer"
)

type grpcWebHandler struct {
	handler http.Handler
}

// Handler returns a http.Handler that wraps a gRPC handler and enables
// the bridging of a gRPC-Web client to gRPC server.
func Handler(h http.Handler) http.Handler {
	return &grpcWebHandler{h}
}

// RootHandler returns a http.Handler that dispatches requests to either a gRPC,
// gRPC-Web or fallback http.Handler.
//
// This is useful when you want to serve gRPC requests (directly or via the web
// handler) whilst also serving regular HTTP requests.
//
// It's worth reading https://godoc.org/google.golang.org/grpc#Server.ServeHTTP
// and its notes about any performance/limitation issues with this approach.
func RootHandler(gRPCHandler http.Handler, fallback http.Handler) http.Handler {
	gRPCWebHandler := Handler(gRPCHandler)

	fn := func(resp http.ResponseWriter, req *http.Request) {
		switch true {
		case IsGRPCWebRequest(req):
			gRPCWebHandler.ServeHTTP(resp, req)

		case IsGRPCRequest(req):
			gRPCHandler.ServeHTTP(resp, req)

		default:
			fallback.ServeHTTP(resp, req)
		}
	}

	return http.HandlerFunc(fn)
}

func (h *grpcWebHandler) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	if !IsGRPCWebRequest(req) {
		h.handler.ServeHTTP(resp, req)
		return
	}

	// convert to HTTP/2 request
	req.ProtoMajor = 2
	req.ProtoMinor = 0

	// ensure chunked encoding
	req.Header.Del(headerContentLength)

	var isTextRequest bool
	switch req.Header.Get(headerContentType) {
	case ContentTypeGRPCWebText, ContentTypeGRPCWebTextProto:
		isTextRequest = true
	}
	req.Header.Set(headerContentType, ContentTypeGRPC)

	var isTextResponse bool
	switch req.Header.Get(headerAccept) {
	case ContentTypeGRPCWebText, ContentTypeGRPCWebTextProto:
		isTextResponse = true
	}

	req.Header.Set(headerTE, "trailers")
	req.Header.Set(headerGRPCAcceptEncoding, "identity,deflate,gzip")

	if isTextRequest {
		req.Body = bodyCloser{base64.NewDecoder(base64.StdEncoding, req.Body), req.Body}
	}

	contentType := ContentTypeGRPCWebProto
	if isTextResponse {
		contentType = ContentTypeGRPCWebTextProto
	}

	// handle request
	resp = &gRPCWebResponseWriter{wrapped: resp, contentType: contentType}
	h.handler.ServeHTTP(resp, req)

	// write trailers
	trailers := make(http.Header)
	for header, val := range resp.Header() {
		if strings.ToLower(header) == headerTrailer {
			for _, trailer := range val {
				field := resp.Header().Get(trailer)
				if field == "" {
					continue
				}

				trailers.Set(trailer, field)
			}
			break
		}
	}

	buf := new(bytes.Buffer)
	trailers.Write(buf)

	resp.Write([]byte{1 << 7})
	binary.Write(resp, binary.BigEndian, uint32(buf.Len()))
	buf.WriteTo(resp)
}

// IsGRPCWebRequest returns true if the request is for a gRPC-Web handler.
func IsGRPCWebRequest(req *http.Request) bool {
	switch req.Header.Get(headerContentType) {
	case
		ContentTypeGRPCWeb,
		ContentTypeGRPCWebProto,
		ContentTypeGRPCWebText,
		ContentTypeGRPCWebTextProto:
		return true

	default:
		return false
	}
}

// IsGRPCRequest returns true if the request is for a gRPC handler.
func IsGRPCRequest(req *http.Request) bool {
	return req.ProtoMajor == 2 && strings.HasPrefix(req.Header.Get(headerContentType), ContentTypeGRPC)
}

type bodyCloser struct {
	io.Reader
	closer io.Closer
}

func (bc bodyCloser) Close() error {
	return bc.closer.Close()
}

type gRPCWebResponseWriter struct {
	wrapped     http.ResponseWriter
	encoder     io.Writer
	contentType string
}

func (w *gRPCWebResponseWriter) Header() http.Header {
	return w.wrapped.Header()
}

func (w *gRPCWebResponseWriter) Write(p []byte) (int, error) {
	if w.encoder == nil {
		w.Header().Set(headerContentType, w.contentType)

		if w.contentType == ContentTypeGRPCWebTextProto {
			w.encoder = base64.NewEncoder(base64.StdEncoding, w.wrapped)
		} else {
			w.encoder = w.wrapped
		}
	}

	return w.encoder.Write(p)
}

func (w *gRPCWebResponseWriter) WriteHeader(statusCode int) {
	w.Header().Set(headerContentType, w.contentType)
	w.wrapped.WriteHeader(statusCode)
}

func (w *gRPCWebResponseWriter) Flush() {
	if wc, ok := w.encoder.(io.WriteCloser); ok {
		wc.Close()
		w.encoder = nil
	}

	w.wrapped.(http.Flusher).Flush()
}

func (w *gRPCWebResponseWriter) CloseNotify() <-chan bool {
	return w.wrapped.(http.CloseNotifier).CloseNotify()
}
