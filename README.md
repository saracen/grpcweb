# grpcweb
[![GoDoc](https://godoc.org/github.com/saracen/grpcweb?status.svg)](https://godoc.org/github.com/saracen/grpcweb)

`grpcweb` is middleware for bridging gRPC-Web clients to a gRPC server.

#### Supports
- base64 encoded payloads (`application/grpc-web-text`, `application/grpc-web-text+proto`)
- binary protobuf payloads (`application/grpc-web`, `application/grpc-web+proto`)
- unary calls
- server streaming calls