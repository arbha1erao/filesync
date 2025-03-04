lint:
	@echo "Running lint on client..."
	@(cd client && golangci-lint run)

	@echo "Running lint on server..."
	@(cd server && golangci-lint run)

	@echo "Linting completed!"

clean:
	rm -rf ./server/server_storage
	rm -rf ./client/local_storage