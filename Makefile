.PHONY: build test consumer tf-init tf-apply tf-destroy publish tf-apply-events tf-destroy-events

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

# System-topic demo: a system topic on a resource group that fires
# Microsoft.Resources.* events when resources in that group are created/deleted.
tf-apply-events:
	locally run terraform -chdir=terraform-system-topic apply

tf-destroy-events:
	locally run terraform -chdir=terraform-system-topic destroy
