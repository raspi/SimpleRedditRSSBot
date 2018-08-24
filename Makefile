LAST_TAG := $(shell git describe --abbrev=0 --always --tags)
BUILD := $(shell git rev-parse $(LAST_TAG))

BINARY := redditrssbot
UNIXBINARY := $(BINARY)-x64
BUILDDIR := build

LINUXRELEASE := $(BINARY)-$(LAST_TAG)-linux-x64.tar.gz

LDFLAGS := -ldflags "-s -w -X=main.VERSION=$(LAST_TAG) -X=main.BUILD=$(BUILD)"

bin:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -v -o $(BUILDDIR)/$(UNIXBINARY)
	upx -v -9 $(BUILDDIR)/$(UNIXBINARY)

release:
	tar cvzf $(BUILDDIR)/$(LINUXRELEASE) $(BUILDDIR)/$(UNIXBINARY)

.PHONY: all clean test

