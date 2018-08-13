package grpcweb_test

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/saracen/grpcweb"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/interop"
	testpb "google.golang.org/grpc/interop/grpc_testing"
)

func TestIsGRPCWebRequest(t *testing.T) {
	supported := []string{
		grpcweb.ContentTypeGRPCWeb,
		grpcweb.ContentTypeGRPCWebProto,
		grpcweb.ContentTypeGRPCWebText,
		grpcweb.ContentTypeGRPCWebTextProto,
	}

	req := &http.Request{}
	req.Header = make(http.Header)
	for _, contentType := range supported {
		req.Header.Set("content-type", contentType)

		assert.True(t, grpcweb.IsGRPCWebRequest(req))
	}

	req.Header.Set("content-type", "unsupported")
	assert.False(t, grpcweb.IsGRPCWebRequest(req))
}

func TestIsGRPCRequest(t *testing.T) {
	req := &http.Request{}
	req.Header = make(http.Header)

	req.ProtoMajor = 1
	req.Header.Set("content-type", "unsupported")
	assert.False(t, grpcweb.IsGRPCRequest(req))

	req.ProtoMajor = 1
	req.Header.Set("content-type", grpcweb.ContentTypeGRPC)
	assert.False(t, grpcweb.IsGRPCRequest(req))

	req.ProtoMajor = 2
	req.Header.Set("content-type", "unsupported")
	assert.False(t, grpcweb.IsGRPCRequest(req))

	req.ProtoMajor = 2
	req.Header.Set("content-type", grpcweb.ContentTypeGRPC)
	assert.True(t, grpcweb.IsGRPCRequest(req))
}

func TestInterop(t *testing.T) {
	server := grpc.NewServer()
	testpb.RegisterTestServiceServer(server, interop.NewTestServer())

	ts := httptest.NewTLSServer(grpcweb.RootHandler(server, http.DefaultServeMux))
	defer ts.Close()

	type requestTest struct {
		Path        string
		ContentType string
		Accept      string
		Request     []byte
		Response    []byte
	}

	requests := []requestTest{
		// emptycall - base64 request, base64 response
		{
			"/grpc.testing.TestService/EmptyCall",
			grpcweb.ContentTypeGRPCWebText,
			grpcweb.ContentTypeGRPCWebText,
			[]byte("AAAAAAA="),
			[]byte("AAAAAAA=gAAAABBHcnBjLVN0YXR1czogMA0K"),
		},
		// emptycall - base64 request, binary response
		{
			"/grpc.testing.TestService/EmptyCall",
			grpcweb.ContentTypeGRPCWebText,
			grpcweb.ContentTypeGRPCWeb,
			[]byte("AAAAAAA="),
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x10, 0x47, 0x72, 0x70, 0x63, 0x2d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x3a, 0x20, 0x30, 0x0d, 0x0a},
		},
		// emptycall - base64 request (no padding, error), binary response
		{
			"/grpc.testing.TestService/EmptyCall",
			grpcweb.ContentTypeGRPCWebText,
			grpcweb.ContentTypeGRPCWeb,
			[]byte("AAAAAAA"),
			[]byte{0x80, 0x00, 0x00, 0x00, 0x2f, 0x47, 0x72, 0x70, 0x63, 0x2d, 0x4d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x3a, 0x20, 0x75, 0x6e, 0x65, 0x78, 0x70, 0x65, 0x63, 0x74, 0x65, 0x64, 0x20, 0x45, 0x4f, 0x46, 0x0d, 0x0a, 0x47, 0x72, 0x70, 0x63, 0x2d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x3a, 0x20, 0x31, 0x33, 0x0d, 0x0a},
		},
		// emptycall - binary request, binary response
		{
			"/grpc.testing.TestService/EmptyCall",
			grpcweb.ContentTypeGRPCWeb,
			grpcweb.ContentTypeGRPCWeb,
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00},
			[]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x10, 0x47, 0x72, 0x70, 0x63, 0x2d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x3a, 0x20, 0x30, 0x0d, 0x0a},
		},
		// unarycall - base64 request, base64 response
		{
			"/grpc.testing.TestService/UnaryCall",
			grpcweb.ContentTypeGRPCWebText,
			grpcweb.ContentTypeGRPCWebText,
			[]byte("AAAAAAQQBSAB"),
			[]byte("AAAAAAkKBxIFAAAAAAA=gAAAABBHcnBjLVN0YXR1czogMA0K"),
		},
		// unarycall - binary request, binary response
		{
			"/grpc.testing.TestService/UnaryCall",
			grpcweb.ContentTypeGRPCWeb,
			grpcweb.ContentTypeGRPCWeb,
			[]byte{0x00, 0x00, 0x00, 0x00, 0x04, 0x10, 0x05, 0x20, 0x01},
			[]byte{0x00, 0x00, 0x00, 0x00, 0x09, 0x0a, 0x07, 0x12, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x10, 0x47, 0x72, 0x70, 0x63, 0x2d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x3a, 0x20, 0x30, 0x0d, 0x0a},
		},
		// streamingoutputcall - base64 request, base64 response
		{
			"/grpc.testing.TestService/StreamingOutputCall",
			grpcweb.ContentTypeGRPCWebText,
			grpcweb.ContentTypeGRPCWebText,
			[]byte("AAAAAAgSAggFEgIICg=="),
			[]byte("AAAAAAkKBxIFAAAAAAA=AAAAAA4KDBIKAAAAAAAAAAAAAA==gAAAABBHcnBjLVN0YXR1czogMA0K"),
		},
		// streamingoutputcall - binary request, binary response
		{
			"/grpc.testing.TestService/StreamingOutputCall",
			grpcweb.ContentTypeGRPCWeb,
			grpcweb.ContentTypeGRPCWeb,
			[]byte{0x00, 0x00, 0x00, 0x00, 0x08, 0x12, 0x02, 0x08, 0x05, 0x12, 0x02, 0x08, 0x0a},
			[]byte{0x00, 0x00, 0x00, 0x00, 0x09, 0x0a, 0x07, 0x12, 0x5, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0e, 0x0a, 0x0c, 0x12, 0x0a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x80, 0x00, 0x00, 0x00, 0x10, 0x47, 0x72, 0x70, 0x63, 0x2d, 0x53, 0x74, 0x61, 0x74, 0x75, 0x73, 0x3a, 0x20, 0x30, 0x0d, 0x0a},
		},
	}

	for _, request := range requests {
		buf := new(bytes.Buffer)
		_, err := buf.Write(request.Request)
		assert.NoError(t, err)

		req, err := http.NewRequest("POST", ts.URL+request.Path, buf)
		assert.NoError(t, err)
		req.Header.Add("content-type", request.ContentType)
		req.Header.Add("accept", request.Accept)

		resp, err := ts.Client().Do(req)
		assert.NoError(t, err)
		assert.Equal(t, resp.StatusCode, http.StatusOK)

		data, err := ioutil.ReadAll(resp.Body)
		assert.NoError(t, err)
		assert.Equal(t, request.Response, data)
	}
}
