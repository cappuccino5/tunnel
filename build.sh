#!/bin/bash
echo "更新依赖文件"
GONOSUMDB="github.com"  GOPRIVATE='github.com*' go mod tidy

#打包windows版本
echo "开始编译"
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build  -o ./bin/tunnel-windows64.exe  main.go

# 打包linux mipsle 版本
CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -o ./bin/tunnel-mpisle  main.go

# 打包linux AMD64 版本
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build  -o ./bin/tunnel-amd64  main.go


# 打包linux ARM64 版本
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build  -o ./bin/tunnel-arm64 main.go

echo "编译完成"