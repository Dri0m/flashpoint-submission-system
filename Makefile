#!make
include .env
export $(shell sed 's/=.*//' .env)

.PHONY: db migrate run

db:
	docker-compose -p fpfssdb -f dc-db.yml down -v
	docker-compose -p fpfssdb -f dc-db.yml up

migrate:
	docker run --rm -v $(shell pwd)/migrations:/migrations --network host migrate/migrate -path=/migrations/ -database "mysql://${DB_USER}:${DB_PASSWORD}@tcp(${DB_IP}:${DB_PORT})/${DB_NAME}" up

validator:
	cd .. && cd Curation-Validation-Bot && python3.9 -m uvicorn validator-server:app --host 127.0.0.1 --port 8371

run:
	rm flashpoint-submission-system | true
	go build ./cmd/
	./flashpoint-submission-system

