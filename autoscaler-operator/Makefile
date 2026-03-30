IMAGE ?= yourorg/autoscaler-operator
TAG   ?= latest

.PHONY: build test docker-build docker-push deploy undeploy fmt vet lint

## Build the operator binary locally.
build:
	go build -o bin/operator ./cmd/operator

## Run all unit tests.
test:
	go test ./... -v -count=1

## Run go fmt.
fmt:
	go fmt ./...

## Run go vet.
vet:
	go vet ./...

## Build the Docker image.
docker-build:
	docker build -t $(IMAGE):$(TAG) .

## Push the Docker image.
docker-push:
	docker push $(IMAGE):$(TAG)

## Deploy RBAC + operator to the current cluster context.
deploy:
	kubectl apply -f config/rbac/clusterrole.yaml
	kubectl apply -f config/manager/deployment.yaml

## Remove operator from the cluster.
undeploy:
	kubectl delete -f config/manager/deployment.yaml --ignore-not-found
	kubectl delete -f config/rbac/clusterrole.yaml  --ignore-not-found

## Apply the example Deployment.
example:
	kubectl apply -f deploy/example-deployment.yaml

## Tail operator logs.
logs:
	kubectl logs -n autoscaler-system -l app.kubernetes.io/name=autoscaler-operator -f
