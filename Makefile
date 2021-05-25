.PHONY: db migrate run

db:
	docker-compose -p fpfssdb -f dc-db.yml down -v
	docker-compose -p fpfssdb -f dc-db.yml up

run:
	rm flashpoint-submission-system | true
	go build ./cmd/
	./flashpoint-submission-system

