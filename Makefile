.PHONY: build test consumer tf-init tf-apply tf-destroy publish

build:
	go build ./...

test:
	go test ./...

consumer:
	go run ./cmd/consumer

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
