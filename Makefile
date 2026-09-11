.PHONY: build test consumer tf-init tf-apply tf-destroy publish

# EventGrid requires an https webhook, and Locally verifies the cert on delivery.
# Locally ships a localhost cert (covering localhost + *.locally) that its own
# delivery client trusts, so point the consumer at it — no cert generation needed.
LOCALLY_CONFIG_DIR ?= $(HOME)/.config/locally
LOCALLY_CERT       ?= $(LOCALLY_CONFIG_DIR)/localhost.pem
LOCALLY_KEY        ?= $(LOCALLY_CONFIG_DIR)/localhost.key

build:
	go build ./...

test:
	go test ./...

consumer:
	go run ./cmd/consumer -tls-cert "$(LOCALLY_CERT)" -tls-key "$(LOCALLY_KEY)"

tf-init:
	locally run terraform -chdir=terraform init

tf-apply:
	locally run terraform -chdir=terraform apply

tf-destroy:
	locally run terraform -chdir=terraform destroy

publish:
	TOPIC_ENDPOINT=$$(locally run terraform -chdir=terraform output -raw topic_endpoint) \
	TOPIC_KEY=$$(locally run terraform -chdir=terraform output -raw topic_primary_access_key) \
	go run ./cmd/publisher
