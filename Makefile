deps:
	go mod download

covertest: deps
	go test  -coverprofile=coverage.out
	go tool cover -html=coverage.out