deps-update:
    go get -u ./...
    go mod tidy

test:
  go test -short -v

full-test:
  go test -v

