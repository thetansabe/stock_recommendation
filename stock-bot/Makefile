.PHONY: run build clean tidy check

run:
	go run .

check:
	go test -v -run TestConsoleCheck -count=1

build:
	go build -o stock-bot .

tidy:
	go mod tidy

clean:
	rm -f stock-bot stock-bot.db
