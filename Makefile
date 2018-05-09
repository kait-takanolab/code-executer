# 参考: https://frasco.io/golang-dont-afraid-of-makefiles-785f3ec7eb32
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=ce.out

all: build
build:
	$(GOBUILD) -o $(BINARY_NAME) -v
run:
	$(GOBUILD) -o $(BINARY_NAME) -v
	./$(BINARY_NAME)
