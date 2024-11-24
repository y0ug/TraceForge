# GOOS=windows GOARCH=amd64 go build -o bin/hvapi.exe ./cmd/api
# GOOS=windows GOARCH=amd64 go build -o bin/test.exe ./cmd/test
GOOS=windows GOARCH=amd64 go build -o bin/hvapi.exe ./cmd/hvapi
GOOS=windows GOARCH=amd64 go build -o bin/hvapi-release.exe -ldflags "-s -w" ./cmd/hvapi
GOOS=windows GOARCH=amd64 go build -o bin/agent-release.exe -ldflags "-s -w" ./cmd/agent
# CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -o bin/MQ.exe ./cmd/MQ
#
swag init --output ./cmd/hvapi/docs/ --parseInternal --parseDependency --dir ./cmd/hvapi,./internals
