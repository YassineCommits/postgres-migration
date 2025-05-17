package main

import (
	"flag"
	"log"
	"os"

	"pg-migration/pkg/config"
	"pg-migration/pkg/replication"
	"pg-migration/pkg/schema"
)

func main() {
	// Set up logging
	log.SetOutput(os.Stdout)
	log.SetPrefix("[PG-MIGRATION] ")

	// Define flags for command-line arguments
	sourceHost := flag.String("source-host", "", "Source PostgreSQL host")
	sourcePort := flag.Int("source-port", 0, "Source PostgreSQL port")
	sourceUser := flag.String("source-user", "", "Source PostgreSQL user")
	sourcePassword := flag.String("source-password", "", "Source PostgreSQL password")
	sourceDB := flag.String("source-db", "", "Source PostgreSQL database name")
	sourceSSLMode := flag.String("source-sslmode", "require", "Source PostgreSQL SSL mode (require, verify-ca, verify-full, disable)")

	targetHost := flag.String("target-host", "", "Target PostgreSQL host")
	targetPort := flag.Int("target-port", 0, "Target PostgreSQL port")
	targetUser := flag.String("target-user", "", "Target PostgreSQL user")
	targetPassword := flag.String("target-password", "", "Target PostgreSQL password")
	targetDB := flag.String("target-db", "", "Target PostgreSQL database name")
	targetSSLMode := flag.String("target-sslmode", "require", "Target PostgreSQL SSL mode (require, verify-ca, verify-full, disable)")

	// Operation flags
	dumpSchema := flag.Bool("dump-schema", false, "Dump schema from source database")
	restoreSchema := flag.Bool("restore-schema", false, "Restore schema to target database")
	schemaFile := flag.String("schema-file", "", "File path for schema dump or restore (optional)")
	setupReplication := flag.Bool("setup-replication", false, "Setup logical replication after migration")
	fullMigration := flag.Bool("full-migration", false, "Perform complete migration (schema dump, restore, and replication setup)")

	flag.Parse()

	// Load configuration from flags or environment variables
	sourceConfig, err := config.LoadSourceConfig(*sourceHost, *sourcePort, *sourceUser, *sourcePassword, *sourceDB, *sourceSSLMode)
	if err != nil {
		log.Fatalf("Failed to load source configuration: %v", err)
	}

	targetConfig, err := config.LoadTargetConfig(*targetHost, *targetPort, *targetUser, *targetPassword, *targetDB, *targetSSLMode)
	if err != nil {
		log.Fatalf("Failed to load target configuration: %v", err)
	}

	// Handle schema operations
	schemaHandler := schema.NewSchemaHandler(sourceConfig, targetConfig)

	// Full migration process
	if *fullMigration {
		log.Println("Starting full migration process...")

		// Step 1: Dump schema from source and restore to target
		log.Println("Step 1: Dumping and restoring schema...")
		if err := schemaHandler.DumpAndRestoreSchema(); err != nil {
			log.Fatalf("Failed to dump and restore schema: %v", err)
		}

		// Step 2: Setup logical replication
		log.Println("Step 2: Setting up logical replication...")
		replicator := replication.NewReplicator(sourceConfig, targetConfig)
		if err := replicator.SetupReplication(); err != nil {
			log.Fatalf("Failed to setup replication: %v", err)
		}

		log.Println("Full migration process completed successfully.")
		return
	}

	// Handle individual operations
	if *dumpSchema {
		log.Println("Dumping schema from source database...")

		if *schemaFile != "" {
			if err := schemaHandler.DumpSchemaToFile(*schemaFile); err != nil {
				log.Fatalf("Failed to dump schema to file: %v", err)
			}
			log.Printf("Schema dumped successfully to file: %s\n", *schemaFile)
		} else {
			// Create a temporary file with timestamp
			tempFile, err := os.CreateTemp("", "schema-dump-*.sql")
			if err != nil {
				log.Fatalf("Failed to create temporary file: %v", err)
			}
			tempFilePath := tempFile.Name()
			tempFile.Close()

			if err := schemaHandler.DumpSchemaToFile(tempFilePath); err != nil {
				log.Fatalf("Failed to dump schema: %v", err)
			}
			log.Printf("Schema dumped successfully to file: %s\n", tempFilePath)
		}
	}

	if *restoreSchema {
		log.Println("Restoring schema to target database...")

		if *schemaFile == "" {
			log.Fatalf("Schema file path is required for restore operation. Use --schema-file flag.")
		}

		if err := schemaHandler.RestoreSchemaFromFile(*schemaFile); err != nil {
			log.Fatalf("Failed to restore schema: %v", err)
		}
		log.Println("Schema restored successfully to target database.")
	}

	// Setup logical replication if requested
	if *setupReplication {
		log.Println("Setting up logical replication...")
		replicator := replication.NewReplicator(sourceConfig, targetConfig)
		if err := replicator.SetupReplication(); err != nil {
			log.Fatalf("Failed to setup replication: %v", err)
		}
		log.Println("Logical replication setup completed successfully.")
	}

	if !*dumpSchema && !*restoreSchema && !*setupReplication && !*fullMigration {
		log.Println("No operation specified. Use --dump-schema, --restore-schema, --setup-replication, or --full-migration.")
		flag.Usage()
	} else {
		log.Println("Migration process completed.")
	}
}
