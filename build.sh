#!/bin/bash
GONOSUMDB="dev.risinghf.com" GOPROXY=https://mirrors.aliyun.com/goproxy/,goproxy.io,direct GOPRIVATE='dev.risinghf.com*' go mod tidy
GONOSUMDB="github.com" GOPROXY=https://mirrors.aliyun.com/goproxy/,goproxy.io,direct GOPRIVATE='github.com*' go mod tidy
go build main.go