package replication

import (
	"database/sql"
	"fmt"
	"log"
	"pg-migration/pkg/config"

	_ "github.com/lib/pq" // PostgreSQL driver
)

// Replicator holds the source and target database configuration.
type Replicator struct {
	source *config.DBConfig
	target *config.DBConfig
}

// NewReplicator creates a new Replicator instance.
func NewReplicator(source, target *config.DBConfig) *Replicator {
	return &Replicator{
		source: source,
		target: target,
	}
}

// checkExtensionInstalled verifies if a given extension (e.g., aiven_extras) is installed.
func checkExtensionInstalled(db *sql.DB, extName string) (bool, error) {
	var exists bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = $1);"
	err := db.QueryRow(query, extName).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// installExtension attempts to install the specified extension in the database.
func installExtension(db *sql.DB, extName string) error {
	query := fmt.Sprintf("CREATE EXTENSION IF NOT EXISTS %s;", extName)
	_, err := db.Exec(query)
	return err
}

// checkWalLevel verifies if wal_level is set to 'logical'
func checkWalLevel(db *sql.DB) (bool, error) {
	var walLevel string
	query := "SHOW wal_level;"
	err := db.QueryRow(query).Scan(&walLevel)
	if err != nil {
		return false, err
	}
	return walLevel == "logical", nil
}


// SetupReplication sets up logical replication between the source and target
// databases using the Aiven Extras extension. It creates a publication on the
// source and a subscription on the target.
//
// NOTE: We have now modified the logic so that if a publication or subscription
// already exists, it is dropped first. This is important for applying schema changes.
func (r *Replicator) SetupReplication() error {
	// Connect to source database.
	srcDB, err := sql.Open("postgres", r.source.ConnectionString())
	if err != nil {
		return fmt.Errorf("failed to connect to source database: %v", err)
	}
	defer srcDB.Close()

	// Check if the aiven_extras extension is installed on the source.
	installed, err := checkExtensionInstalled(srcDB, "aiven_extras")
	if err != nil {
		return fmt.Errorf("failed to check aiven_extras extension on source: %v", err)
	}

	// Install aiven_extras if not found.
	if !installed {
		log.Println("aiven_extras extension not found on source database, attempting to install it...")
		if err := installExtension(srcDB, "aiven_extras"); err != nil {
			return fmt.Errorf("failed to install aiven_extras extension on source: %v", err)
		}
		log.Println("Successfully installed aiven_extras extension on source database.")
	}




	// Define the publication name.
	pubName := "aiven_db_migrate_pub"

	// FIX: Instead of skipping creation if the publication exists,
	// we drop any existing publication to account for schema changes.
	dropPubQuery := fmt.Sprintf("DROP PUBLICATION IF EXISTS %s;", pubName)
	if _, err := srcDB.Exec(dropPubQuery); err != nil {
		return fmt.Errorf("failed to drop existing publication: %v", err)
	}
	log.Printf("Dropped existing publication '%s' (if any) on source database.", pubName)

	// Create publication on the source using the Aiven Extras function.
	createPubQuery := `
		SELECT * FROM aiven_extras.pg_create_publication_for_all_tables($1, 'INSERT,UPDATE,DELETE');
	`
	if _, err := srcDB.Exec(createPubQuery, pubName); err != nil {
		return fmt.Errorf("failed to create publication on source: %v", err)
	}
	log.Printf("Publication '%s' created on source database.", pubName)

	// Connect to target database.
	tgtDB, err := sql.Open("postgres", r.target.ConnectionString())
	if err != nil {
		return fmt.Errorf("failed to connect to target database: %v", err)
	}
	defer tgtDB.Close()

	// Check if the aiven_extras extension is installed on the target.
	installed, err = checkExtensionInstalled(tgtDB, "aiven_extras")
	if err != nil {
		return fmt.Errorf("failed to check aiven_extras extension on target: %v", err)
	}

	// Install aiven_extras on target if not found.
	if !installed {
		log.Println("aiven_extras extension not found on target database, attempting to install it...")
		if err := installExtension(tgtDB, "aiven_extras"); err != nil {
			return fmt.Errorf("failed to install aiven_extras extension on target: %v", err)
		}
		log.Println("Successfully installed aiven_extras extension on target database.")
	}

	// Check if wal_level is set to 'logical' on target.
	_, err = checkWalLevel(tgtDB)
	if err != nil {
		return fmt.Errorf("failed to check wal_level on target: %v", err)
	}

	

	// Define the subscription and slot names.
	subName := "aiven_db_migrate_sub"
	slotName := "aiven_db_migrate_slot"

	// FIX: Instead of skipping subscription creation if one exists,
	// we drop any existing subscription to apply the new schema changes.
	dropSubQuery := fmt.Sprintf("DROP SUBSCRIPTION IF EXISTS %s;", subName)
	if _, err := tgtDB.Exec(dropSubQuery); err != nil {
		return fmt.Errorf("failed to drop existing subscription: %v", err)
	}
	log.Printf("Dropped existing subscription '%s' (if any) on target database.", subName)

	// Create subscription on the target using the Aiven Extras function.
	sourceConnStrForSub := r.source.ConnectionString() // Source connection string for subscription.
	createSubQuery := `
		SELECT * FROM aiven_extras.pg_create_subscription($1, $2, $3, $4, true, true);
	`
	if _, err := tgtDB.Exec(createSubQuery, subName, sourceConnStrForSub, pubName, slotName); err != nil {
		return fmt.Errorf("failed to create subscription on target: %v", err)
	}

	log.Printf("Subscription '%s' created on target database. Initial data copy should now be in progress.\n", subName)
	return nil
}
