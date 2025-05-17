
---

# PostgreSQL Migration Tool

A Go utility for migrating PostgreSQL databases that supports both schema transfer and logical replication. This tool streamlines moving data between PostgreSQL instances while ensuring data consistency using Aiven’s logical replication features.

## Features

- **Flexible Configuration:** Configure source and target PostgreSQL connections via command-line flags or environment variables.
- **Schema Operations:** Dump and restore database schemas (tables, views, functions, etc.) between PostgreSQL instances.
- **Logical Replication:** Set up logical replication that creates a publication on the source and a corresponding subscription on the target, including replication slot management and initial data copy.
- **Full Migration Workflow:** Combine schema transfer and replication setup in a single step using the `--full-migration` flag.
- **SSL Support:** Configure SSL modes for secure connections between databases.
- **Aiven Extras Integration:** Leverage Aiven’s PostgreSQL extras extension for simplified replication setup.

## Prerequisites

- **Go 1.15 or higher**
- **PostgreSQL Client Tools:** `pg_dump` and `psql` must be installed and available in your PATH.
- **PostgreSQL Instances:** Both source and target instances must support logical replication.
- **Aiven Extras Extension:** Installed on both the source and target databases.
- **SSL Configuration:** Ensure that the appropriate SSL certificates are configured if using `verify-ca` or `verify-full` modes.

## Installation

Clone the repository and build the application:

```bash
# Clone the repository
git clone https://github.com/yourusername/pg-migration.git
cd pg-migration

# Build the application
go build -o pg-migrate cmd/migrate/main.go
```

## Usage

You can configure the tool either with command-line flags or via environment variables.

### Full Migration Example

This command performs a complete migration: it dumps the schema from the source, restores it to the target, and then sets up logical replication.

```bash
./pg-migrate \
  --source-host=source.example.com \
  --source-port=5432 \
  --source-user=postgres \
  --source-password=password \
  --source-db=sourcedb \
  --source-sslmode=require \
  --target-host=target.example.com \
  --target-port=5432 \
  --target-user=postgres \
  --target-password=password \
  --target-db=targetdb \
  --target-sslmode=require \
  --full-migration
```

### Environment Variables Example

Set environment variables to define connection parameters, then run the tool:

```bash
export SOURCE_DB_HOST=source.example.com
export SOURCE_DB_PORT=5432
export SOURCE_DB_USER=postgres
export SOURCE_DB_PASSWORD=password
export SOURCE_DB_NAME=sourcedb
export SOURCE_DB_SSLMODE=require

export TARGET_DB_HOST=target.example.com
export TARGET_DB_PORT=5432
export TARGET_DB_USER=postgres
export TARGET_DB_PASSWORD=password
export TARGET_DB_NAME=targetdb
export TARGET_DB_SSLMODE=require

./pg-migrate --full-migration
```

### Individual Operations

- **Dump Schema:** Extract the schema from the source database.
  
  ```bash
  ./pg-migrate --source-host=source.example.com \
    --source-port=5432 \
    --source-user=postgres \
    --source-password=password \
    --source-db=sourcedb \
    --source-sslmode=require \
    --dump-schema --schema-file=./schema.sql
  ```

- **Restore Schema:** Apply a dumped schema to the target database.

  ```bash
  ./pg-migrate --target-host=target.example.com \
    --target-port=5432 \
    --target-user=postgres \
    --target-password=password \
    --target-db=targetdb \
    --target-sslmode=require \
    --restore-schema --schema-file=./schema.sql
  ```

- **Setup Logical Replication:** Configure replication between the source and target databases.

  ```bash
  ./pg-migrate --source-host=source.example.com \
    --source-port=5432 \
    --source-user=postgres \
    --source-password=password \
    --source-db=sourcedb \
    --source-sslmode=require \
    --target-host=target.example.com \
    --target-port=5432 \
    --target-user=postgres \
    --target-password=password \
    --target-db=targetdb \
    --target-sslmode=require \
    --setup-replication
  ```

## SSL Configuration

The tool supports secure connections via SSL. Both source and target configurations include an SSL mode option. You can choose from the following modes:

- **require:** Enforces SSL without verifying certificates (default).
- **verify-ca:** Verifies the certificate authority.
- **verify-full:** Performs full verification of the SSL certificate.
- **disable:** Disables SSL.

Use the `--source-sslmode` and `--target-sslmode` flags (or the environment variables `SOURCE_DB_SSLMODE` and `TARGET_DB_SSLMODE`) to specify the desired SSL mode.

## Command-line Options

| Flag                  | Environment Variable  | Description                                                            |
|-----------------------|-----------------------|------------------------------------------------------------------------|
| `--source-host`       | `SOURCE_DB_HOST`      | Source PostgreSQL host                                                 |
| `--source-port`       | `SOURCE_DB_PORT`      | Source PostgreSQL port (default: 5432)                                 |
| `--source-user`       | `SOURCE_DB_USER`      | Source PostgreSQL user                                                 |
| `--source-password`   | `SOURCE_DB_PASSWORD`  | Source PostgreSQL password                                             |
| `--source-db`         | `SOURCE_DB_NAME`      | Source PostgreSQL database name                                        |
| `--source-sslmode`    | `SOURCE_DB_SSLMODE`   | Source PostgreSQL SSL mode (require, verify-ca, verify-full, disable)    |
| `--target-host`       | `TARGET_DB_HOST`      | Target PostgreSQL host                                                 |
| `--target-port`       | `TARGET_DB_PORT`      | Target PostgreSQL port (default: 5432)                                 |
| `--target-user`       | `TARGET_DB_USER`      | Target PostgreSQL user                                                 |
| `--target-password`   | `TARGET_DB_PASSWORD`  | Target PostgreSQL password                                             |
| `--target-db`         | `TARGET_DB_NAME`      | Target PostgreSQL database name                                        |
| `--target-sslmode`    | `TARGET_DB_SSLMODE`   | Target PostgreSQL SSL mode (require, verify-ca, verify-full, disable)    |
| `--dump-schema`       | -                     | Dump schema from the source database                                   |
| `--restore-schema`    | -                     | Restore schema to the target database                                  |
| `--schema-file`       | -                     | File path for schema dump or restore (optional)                        |
| `--setup-replication` | -                     | Set up logical replication                                             |
| `--full-migration`    | -                     | Perform complete migration (schema dump, restore, and replication setup)|

## Migration Operations

### Schema Operations

- **Dump Schema:**  
  Extracts only the schema (tables, views, functions, etc.) from the source database. If no file path is provided via `--schema-file`, a temporary file is created.

- **Restore Schema:**  
  Applies the dumped schema to the target database. Requires the path to the schema file to be specified.

### Logical Replication Process

When you run the tool with the `--setup-replication` flag, it will:
1. Create a publication (`aiven_db_migrate_pub`) on the source database.
2. Create a subscription (`aiven_db_migrate_sub`) on the target database.
3. Set up a replication slot and initiate an initial data copy.
4. Establish ongoing replication, ensuring that changes on the source are propagated to the target.

### Full Migration

Using the `--full-migration` flag, the tool performs:
1. A schema dump from the source.
2. A schema restore to the target.
3. Logical replication setup to maintain data consistency.
4. Verification of successful data copy and replication status.

## Testing

The repository includes testing scripts such as `reset-env.sh` to help you set up a controlled testing environment. Use these scripts to verify your migration process before running in production.

## Project Structure

```
.
├── cmd
│   └── migrate
│       └── main.go         # Main application entry point
├── pkg
│   ├── config
│   │   └── config.go       # Database configuration handling (flags & environment variables)
│   ├── replication
│   │   └── replication.go  # Logical replication setup and management
│   └── schema
│       └── schema.go       # Schema dump and restore operations
├── go.mod
├── go.sum
├── README.md
└── reset-env.sh            # Testing environment setup script
```

## Development

To contribute:
1. Fork the repository.
2. Create a feature branch.
3. Implement your changes.
4. Submit a pull request.

## Acknowledgments

- This tool leverages Aiven’s PostgreSQL extras extension for simplified replication setup.
- Thanks to the PostgreSQL community for providing robust tools and extensive support.

---

