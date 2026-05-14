package shortenerv1

//go:generate protoc --proto_path=../../third_party --proto_path=. shortener.proto --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:.
