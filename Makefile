BINARY=scripty
INSTALL_DIR=/usr/local/bin

.PHONY: all build install clean

all: build

build:
	go build -o $(BINARY) ./cmd/scripty/

install: build
	install -m 755 $(BINARY) $(INSTALL_DIR)/$(BINARY)
	@echo "Installed $(BINARY) to $(INSTALL_DIR)/$(BINARY)"

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY)
	@echo "Removed $(INSTALL_DIR)/$(BINARY)"

clean:
	rm -f $(BINARY)
