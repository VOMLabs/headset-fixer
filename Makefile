BINARY=scripty

.PHONY: all build install uninstall clean

all: build

build:
	go build -o $(BINARY) ./cmd/scripty/

install: build
	./$(BINARY) install

uninstall:
	rm -f ~/.local/bin/$(BINARY)
	@echo "Removed ~/.local/bin/$(BINARY)"

clean:
	rm -f $(BINARY)
