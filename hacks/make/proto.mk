PROTO_DIR := proto
PROTO_OUT := pkg/rpc
IDENTITY_PROTO_SOURCE := ../indentity/proto/identity/v1/identity.proto
IDENTITY_PROTO_COPY := $(PROTO_DIR)/identity/v1/identity.proto

.PHONY: proto proto-sync proto-check

# Regera o código do gRPC interno a partir de proto/. O código gerado é
# versionado: nunca se gera no boot nem no build.
# vault.proto é o servidor deste serviço e o vault é dono dele (os clientes
# copiam esse arquivo). identity.proto é só o cliente: é uma CÓPIA do contrato
# do identity (o dono), atualizada por `make proto-sync`; a opção M troca o
# go_package dele (que aponta para o módulo do identity) para cá.
# Precisa de protoc, protoc-gen-go e protoc-gen-go-grpc no PATH.
proto:
	@rm -rf $(PROTO_OUT)/vaultv1 $(PROTO_OUT)/identityv1 && mkdir -p $(PROTO_OUT)/vaultv1 $(PROTO_OUT)/identityv1
	protoc -I $(PROTO_DIR) \
		--go_out=$(PROTO_OUT) --go_opt=paths=source_relative \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative \
		$(PROTO_DIR)/vault/v1/vault.proto
	protoc -I $(PROTO_DIR) \
		--go_out=$(PROTO_OUT) --go_opt=paths=source_relative,Midentity/v1/identity.proto=github.com/murkio-dev/vault/pkg/rpc/identityv1 \
		--go-grpc_out=$(PROTO_OUT) --go-grpc_opt=paths=source_relative,Midentity/v1/identity.proto=github.com/murkio-dev/vault/pkg/rpc/identityv1 \
		$(IDENTITY_PROTO_COPY)
	@mv $(PROTO_OUT)/vault/v1/*.go $(PROTO_OUT)/vaultv1/ && mv $(PROTO_OUT)/identity/v1/*.go $(PROTO_OUT)/identityv1/
	@rm -rf $(PROTO_OUT)/vault $(PROTO_OUT)/identity

# Copia o contrato do identity (o dono) para cá. Rode antes de `make proto`
# quando o identity mudar o identity.proto.
proto-sync:
	cp $(IDENTITY_PROTO_SOURCE) $(IDENTITY_PROTO_COPY)

# Falha se a cópia do identity.proto ficou diferente do original do identity.
proto-check:
	diff -u $(IDENTITY_PROTO_SOURCE) $(IDENTITY_PROTO_COPY)
