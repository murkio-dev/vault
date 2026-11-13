PROTO_DIR := ../proto
PROTO_OUT := pkg/rpc

.PHONY: proto

# Regera o código do gRPC interno a partir de ../proto (fonte compartilhada pelos
# serviços). O código gerado é versionado: nunca se gera no boot nem no build.
# Precisa de protoc, protoc-gen-go e protoc-gen-go-grpc no PATH.
proto:
	@rm -rf $(PROTO_OUT)/vaultv1 && mkdir -p $(PROTO_OUT)/vaultv1
	protoc -I $(PROTO_DIR) \
		--go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/vault/v1/vault.proto
	@mv $(PROTO_OUT)/vault/v1/*.go $(PROTO_OUT)/vaultv1/ && rm -rf $(PROTO_OUT)/vault
