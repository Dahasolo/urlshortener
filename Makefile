# По умолчанию — запуск
default: run

.PHONY: default help build run test clean migrate.up migrate.down

help:
	@echo "Доступные команды:"
	@echo "  make              - собрать и запустить"
	@echo "  make build        - собрать бинарник"
	@echo "  make run          - запустить (если уже собран)"
	@echo "  make test         - запустить тесты"
	@echo "  make clean        - удалить бинарник"
	@echo "  make migrate.up   - применить все миграции"
	@echo "  make migrate.down - откатить все миграции"

# Сборка
build:
	cd ./cmd/shortener && go build -o shortener .

# Запуск
run: build
	./cmd/shortener/shortener

# Тесты
test:
	go test -v ./...

# Очистка
clean:
	rm -f ./cmd/shortener/shortener

# Генерация мока из .mockery.yaml
mock:
	mockery --config .mockery.yaml

# Генерация нового мока Handler для тестирования gzip
mock_handler:
	mockery --all=false --dir=internal/handler/middleware --name=Handler \
	--output=internal/mocks --filename=mock_handler.go \
	--with-expecter --structname=HandlerMock --log-level=info

# Создание миграции
migrate.create:
	migrate create -ext sql -dir ./migrations $(name)

# Применение всех миграций
migrate.up:
	migrate -database "$(DATABASE_DSN)" -path ./migrations up

# Откат всех миграций
migrate.down:
	migrate -database "$(DATABASE_DSN)" -path ./migrations down
