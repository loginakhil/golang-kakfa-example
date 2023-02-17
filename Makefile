all:
	docker run -v ${PWD}/protos/defs:/defs -v ${PWD}/protos/gen:/gen namely/protoc-all -l go -o /gen -d /defs

start-test-containers:
	docker-compose up --force-recreate