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

dump-db:
	mkdir -p ./backups/db/
	docker exec fpfssdb /usr/bin/mysqldump -u root --password=${DB_ROOT_PASSWORD} ${DB_NAME} > ./backups/db/db-dump-${DB_NAME}-$(shell date -u +"%Y-%m-%d-%H-%M-%S").sql

restore-db:
	cat $(SQL_FILE) | docker exec -i fpfssdb /usr/bin/mysql -u root --password=${DB_ROOT_PASSWORD} ${DB_NAME}

run:
	rm flashpoint-submission-system | true
	go build ./cmd/
	./flashpoint-submission-system

