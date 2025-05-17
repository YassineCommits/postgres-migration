#!/bin/bash
set -e

echo "===== Resetting PostgreSQL Migration Test Environment ====="
echo "Stopping and removing containers..."

# Variables
SOURCE_NAME="postgres-source"
TARGET_NAME="postgres-target"
NETWORK_NAME="pg-migration-network"
SOURCE_PORT=15432
TARGET_PORT=25432
PASSWORD="postgres123"
TEST_DB="testdb"
AIVEN_EXTRAS_REPO="https://github.com/aiven/aiven-extras.git"
AIVEN_EXTRAS_DIR="/tmp/aiven-extras"

# Remove the containers
docker rm -f $SOURCE_NAME $TARGET_NAME 2>/dev/null || true

# Remove the network
docker network rm $NETWORK_NAME 2>/dev/null || true

# Remove temp files
rm -f /tmp/postgres_dump.sql
rm -rf $AIVEN_EXTRAS_DIR

echo "Starting fresh setup..."

# Create Docker network
echo "Creating Docker network..."
docker network create $NETWORK_NAME

# Start source PostgreSQL container with logical replication enabled
echo "Starting source PostgreSQL container on port $SOURCE_PORT..."
docker run --name $SOURCE_NAME \
  --network $NETWORK_NAME \
  -e POSTGRES_PASSWORD=$PASSWORD \
  -p $SOURCE_PORT:5432 \
  -d postgres:13 \
  -c wal_level=logical \
  -c max_wal_senders=10 \
  -c max_replication_slots=10

# Start target PostgreSQL container
echo "Starting target PostgreSQL container on port $TARGET_PORT..."
docker run --name $TARGET_NAME \
  --network $NETWORK_NAME \
  -e POSTGRES_PASSWORD=$PASSWORD \
  -p $TARGET_PORT:5432 \
  -d postgres:13 \
  -c wal_level=logical \
  -c max_wal_senders=10 \
  -c max_replication_slots=10

echo "Waiting for PostgreSQL containers to start up..."
sleep 10

# Drop the target test database if it exists and create a new one
echo "Resetting target database..."
docker exec -i $TARGET_NAME psql -U postgres << EOF
DROP DATABASE IF EXISTS $TEST_DB;
CREATE DATABASE $TEST_DB;
EOF

# Create test database on source
echo "Creating test database on source..."
docker exec -i $SOURCE_NAME psql -U postgres << EOF
CREATE DATABASE $TEST_DB;
EOF

# Clone Aiven Extras repository
echo "Cloning Aiven Extras repository..."
git clone $AIVEN_EXTRAS_REPO $AIVEN_EXTRAS_DIR

# Install Aiven Extras in the source database by building and installing it inside the container
echo "Installing Aiven Extras in the source database..."
docker cp $AIVEN_EXTRAS_DIR $SOURCE_NAME:/aiven-extras
docker exec -i $SOURCE_NAME bash << EOF
apt-get update && apt-get install -y build-essential postgresql-server-dev-13 git
cd /aiven-extras
make && make install
psql -U postgres -d $TEST_DB -c "CREATE EXTENSION aiven_extras;"
EOF

# Install Aiven Extras in the target database
echo "Installing Aiven Extras in the target database..."
docker cp $AIVEN_EXTRAS_DIR $TARGET_NAME:/aiven-extras
docker exec -i $TARGET_NAME bash << EOF
apt-get update && apt-get install -y build-essential postgresql-server-dev-13 git
cd /aiven-extras
make && make install
psql -U postgres -d $TEST_DB -c "CREATE EXTENSION aiven_extras;"
EOF

# Create test schema and populate with sample data in the source database
echo "Creating test schema and sample data..."
docker exec -i $SOURCE_NAME psql -U postgres -d $TEST_DB << EOF
-- Create test tables
CREATE TABLE customers (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    email VARCHAR(100) UNIQUE NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock INT NOT NULL DEFAULT 0
);

CREATE TABLE orders (
    id SERIAL PRIMARY KEY,
    customer_id INT REFERENCES customers(id),
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    total_amount DECIMAL(10, 2) NOT NULL
);

CREATE TABLE order_items (
    id SERIAL PRIMARY KEY,
    order_id INT REFERENCES orders(id),
    product_id INT REFERENCES products(id),
    quantity INT NOT NULL,
    price DECIMAL(10, 2) NOT NULL
);

-- Insert sample data
INSERT INTO customers (name, email) VALUES
('John Doe', 'john@example.com'),
('Jane Smith', 'jane@example.com'),
('Bob Johnson', 'bob@example.com'),
('Alice Brown', 'alice@example.com'),
('Charlie Davis', 'charlie@example.com');

INSERT INTO products (name, price, stock) VALUES
('Laptop', 999.99, 10),
('Smartphone', 599.99, 20),
('Headphones', 99.99, 50),
('Monitor', 299.99, 15),
('Keyboard', 49.99, 30);

INSERT INTO orders (customer_id, total_amount) VALUES
(1, 1599.98),
(2, 699.98),
(3, 149.98),
(4, 899.98),
(5, 349.98);

INSERT INTO order_items (order_id, product_id, quantity, price) VALUES
(1, 1, 1, 999.99),
(1, 3, 1, 99.99),
(2, 2, 1, 599.99),
(2, 5, 1, 49.99),
(3, 3, 1, 99.99),
(3, 5, 1, 49.99),
(4, 1, 1, 999.99),
(5, 4, 1, 299.99),
(5, 5, 1, 49.99);

-- Create a view for demonstration
CREATE VIEW order_summary AS
SELECT 
    o.id AS order_id,
    c.name AS customer_name,
    o.order_date,
    o.total_amount,
    COUNT(oi.id) AS total_items
FROM orders o
JOIN customers c ON o.customer_id = c.id
JOIN order_items oi ON o.id = oi.order_id
GROUP BY o.id, c.name, o.order_date, o.total_amount;

-- Create a function for demonstration
CREATE OR REPLACE FUNCTION calculate_order_total(order_id INT) 
RETURNS DECIMAL AS \$\$
DECLARE
    total DECIMAL(10,2);
BEGIN
    SELECT SUM(price * quantity) INTO total
    FROM order_items
    WHERE order_id = calculate_order_total.order_id;
    
    RETURN total;
END;
\$\$ LANGUAGE plpgsql;
EOF

echo "Verifying data in source database..."
docker exec -i $SOURCE_NAME psql -U postgres -d $TEST_DB -c "SELECT COUNT(*) FROM customers;"
docker exec -i $SOURCE_NAME psql -U postgres -d $TEST_DB -c "SELECT COUNT(*) FROM products;"
docker exec -i $SOURCE_NAME psql -U postgres -d $TEST_DB -c "SELECT COUNT(*) FROM orders;"

# Create .env file for the migration tool
echo "Creating .env file for the migration tool..."
cat > .env << EOF
# Source database configuration
SOURCE_DB_HOST=localhost
SOURCE_DB_PORT=$SOURCE_PORT
SOURCE_DB_USER=postgres
SOURCE_DB_PASSWORD=$PASSWORD
SOURCE_DB_NAME=$TEST_DB

# Target database configuration
TARGET_DB_HOST=localhost
TARGET_DB_PORT=$TARGET_PORT
TARGET_DB_USER=postgres
TARGET_DB_PASSWORD=$PASSWORD
TARGET_DB_NAME=$TEST_DB
EOF

echo "===== Reset Complete ====="
echo "Test environment has been reset and is ready for testing"
echo "Source PostgreSQL is running on port $SOURCE_PORT"
echo "Target PostgreSQL is running on port $TARGET_PORT"
echo "Test database '$TEST_DB' is populated with sample data"
echo "Aiven Extras has been installed in both source and target databases"
echo ""
echo "You can now run your tests again."
