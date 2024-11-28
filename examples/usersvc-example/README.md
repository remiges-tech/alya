# User Service Example

This is a sample microservice built using Alya framework that demonstrates user management functionality with dynamic configuration using Rigel.

## Prerequisites

- Go 1.19 or later
- Docker and Docker Compose
- etcd (provided via Docker Compose)
- PostgreSQL (provided via Docker Compose)

## Project Structure

```
usersvc-example/
├── docker-compose.yaml    # Docker composition for etcd and PostgreSQL  
├── main.go               # Main application entry point
├── setup-config.sh       # Script to initialize Rigel configuration
├── usersvc-schema.json   # Configuration schema for Rigel
└── userservice/          # User service implementation
```

## Setup and Configuration

1. Start the required services:
   ```bash
   docker compose up -d
   ```

2. Run the setup script to initialize configuration:
   ```bash
   ./setup-config.sh
   ```
   This script:
   - Waits for etcd to be ready
   - Loads the configuration schema
   - Sets up database configuration
   - Configures validation rules

3. Build and run the service:
   ```bash
   go run .
   ```

## Configuration Details

The service uses Rigel for dynamic configuration management. Key configuration parameters include:

### Database Configuration
- Host: localhost
- Port: 5432
- User: alyatest
- Password: alyatest
- Database: alyatest

## Development

1. Make sure etcd is running before starting the service
2. The setup script must be run at least once to initialize configuration
3. Any configuration changes can be made using the `rigelctl` command-line tool

## Troubleshooting

1. If the service fails to start, ensure:
   - etcd is running and accessible
   - PostgreSQL is running
   - Configuration is properly set in etcd
   - All required environment variables are set

2. To check configuration values:
   ```bash
   rigelctl --app alya --module usersvc --version 1 --config dev config get <key>
   ```
