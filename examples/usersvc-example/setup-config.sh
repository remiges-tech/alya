#!/bin/bash

# Wait for etcd to be ready
echo "Waiting for etcd to be ready..."
while ! nc -z localhost 2379; do   
  sleep 1
done
echo "etcd is ready!"

# Install Rigel CLI if not already installed
if ! command -v rigelctl &> /dev/null; then
    echo "Installing Rigel CLI..."
    go install github.com/remiges-tech/rigel/cmd/rigelctl@latest
fi

# Function to set config with retry
set_config() {
    local key=$1
    local value=$2
    local max_retries=5
    local retry_count=0

    while [ $retry_count -lt $max_retries ]; do
        if rigelctl --app alya --module usersvc --version 1 --config dev config set "$key" "$value"; then
            echo "Successfully set $key = $value"
            return 0
        else
            retry_count=$((retry_count + 1))
            echo "Failed to set $key, retrying... ($retry_count/$max_retries)"
            sleep 2
        fi
    done

    echo "Failed to set $key after $max_retries attempts"
    return 1
}

# First, load the schema
echo "Loading schema..."
if rigelctl --app alya --module usersvc --version 1 schema add usersvc-schema.json; then
    echo "Schema loaded successfully"
else
    echo "Failed to load schema"
    exit 1
fi

echo "Setting up configuration in etcd..."

# Database configuration
set_config "database.host" "localhost"
set_config "database.port" 5432
set_config "database.user" "alyatest"
set_config "database.password" "alyatest"
set_config "database.dbname" "alyatest"

# Server configuration
set_config "server.port" "8080"

# Validation rules
set_config "validation.name.minLength" "2"
set_config "validation.name.maxLength" "50"
set_config "validation.username.minLength" "3"
set_config "validation.username.maxLength" "30"
set_config "validation.email.maxLength" "100"

echo "Configuration setup complete!"