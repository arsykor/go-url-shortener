package shortenerv1

//go:generate protoc --proto_path=. shortener.proto --go_out=paths=source_relative:. --go_opt=paths=source_relative --go_opt=default_api_level=API_OPAQUE --go-grpc_out=paths=source_relative:. --go-grpc_opt=paths=source_relative
