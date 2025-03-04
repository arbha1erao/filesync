# filesync

**filesync** is a simple, real-time file synchronization tool leveraging WebSockets to keep files synchronized between a server and multiple clients.

🚧 WIP Project: Currently works best with a single client.

## Features

- **Real-time Synchronization** – Automatically synchronizes files between server and clients (currently a client) 
- **WebSocket-based Communication** – Enables efficient, event-driven file updates

## Prerequisites

- Go (version 1.22 or higher)
- Make utility

## Installation

```bash
git clone https://github.com/arbha1erao/filesync.git
cd filesync
make build
```

## Getting Started

### Running the Server

```bash
./build/server
```

When you start the server, a new directory `./server/server_storage` will be created. This serves as the central storage location, and its state will be broadcast to all connected clients.

### Running Clients

In a new terminal window:

```bash
./build/client
```

Each client creates a `./client/local_storage` directory that is actively monitored for changes.

## Cleanup

### Remove Storage Directories

```bash
make clean-storage
```

This command removes both `server_storage` and `local_storage` directories.

### Remove Build Files

```bash
make clean
```

Removes all compiled build artifacts.

## Troubleshooting

- Ensure the server is running before starting a client
