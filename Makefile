.PHONY: lint clean build

lint:
	@echo "Running lint on client..."
	@(cd client && golangci-lint run)

	@echo "Running lint on server..."
	@(cd server && golangci-lint run)

	@echo "Linting completed!"

clean-storage:
	rm -rf ./server/server_storage
	rm -rf ./client/local_storage

clean:
	@echo "Cleaning up build files..."
	rm -rf build/
	@echo "Cleanup completed!"

build:
	@echo "Creating build directory..."
	mkdir -p build
	@echo "Building server..."
	cd server/cmd && go build -o ../../build/server
	@echo "Building client..."
	cd client/cmd && go build -o ../../build/client
	@echo "Build completed! Binaries are in the 'build' directory."
