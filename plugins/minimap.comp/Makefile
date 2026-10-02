WIT_DIR := wit
WIT_PACKAGE := gams:minimap@1.0.0.wasm
WIT_WORLD := generator
WIT_SOURCES := $(shell find $(WIT_DIR) -name '*.wit')
WIT_STAMP := .wit-package.stamp
BINDGEN_STAMP := .wit-bindgen.stamp
WASM_OUT := minimap.comp.wasm

GO ?= go
TINYGO ?= tinygo
WKG ?= wkg

export GOTOOLCHAIN ?= local

.PHONY: all clean wit bindgen wasm test

all: $(WASM_OUT)

wit: $(WIT_STAMP)

bindgen: $(BINDGEN_STAMP)

wasm: $(WASM_OUT)

test:
	$(GO) test ./...

$(WIT_STAMP): $(WIT_SOURCES)
	$(WKG) wit build
	test -f '$(WIT_PACKAGE)'
	touch $@

$(BINDGEN_STAMP): $(WIT_STAMP) go.mod
	$(GO) tool wit-bindgen-go generate --world $(WIT_WORLD) --out internal './$(WIT_PACKAGE)'
	touch $@

$(WASM_OUT): main.go go.mod go.sum $(WIT_STAMP) $(BINDGEN_STAMP)
	$(TINYGO) build -target=wasip2 \
		-o $@ \
		--wit-package '$(WIT_PACKAGE)' \
		--wit-world $(WIT_WORLD) main.go

clean:
	rm -f '$(WIT_PACKAGE)' $(WASM_OUT) $(WIT_STAMP) $(BINDGEN_STAMP)
	rm -rf internal
