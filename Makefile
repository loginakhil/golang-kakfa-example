
PROTOC_COMPILER=docker run -v ${PWD}/protos/defs:/defs -v ${PWD}/protos/gen:/gen namely/protoc-all -l go -o /gen

all:
	$(PROTOC_COMPILER) -d /defs/cpu
	$(PROTOC_COMPILER) -d /defs/ecommerce

start-test-containers:
	docker-compose up --force-recreate