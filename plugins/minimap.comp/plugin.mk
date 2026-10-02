PLUGIN_WASM_COMPONENT := 1
PLUGIN_WIT_WORLD := generator
PLUGIN_WIT_PACKAGE := gams:minimap@1.0.0.wasm
PLUGIN_GO_BINDINGS_OUT := internal
PLUGIN_GO_COMPONENT_MAIN := main.go
PLUGIN_GO_COMPONENT_EXTRA_DEPS := \
  $(PLUGIN_PATH)/plugin.mk \
  $(wildcard $(PLUGIN_PATH)/placement/*.go)
