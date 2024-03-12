#!/bin/bash
GONOSUMDB="dev.risinghf.com" GOPROXY=https://mirrors.aliyun.com/goproxy/,goproxy.io,direct GOPRIVATE='dev.risinghf.com*' go mod tidy
GONOSUMDB="github.com" GOPROXY=https://mirrors.aliyun.com/goproxy/,goproxy.io,direct GOPRIVATE='github.com*' go mod tidy

#打包windows版本
go build main.go

# 打包linux mipsle 版本
CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build