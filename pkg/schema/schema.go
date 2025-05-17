package schema

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"pg-migration/pkg/config"
)

// SchemaHandler manages schema dump and restore operations
type SchemaHandler struct {
	source *config.DBConfig
	target *config.DBConfig
}

// NewSchemaHandler creates a new SchemaHandler instance
func NewSchemaHandler(source, target *config.DBConfig) *SchemaHandler {
	return &SchemaHandler{
		source: source,
		target: target,
	}
}

// DumpAndRestoreSchema performs a schema-only dump from the source database
// and restores it to the target database
func (s *SchemaHandler) DumpAndRestoreSchema() error {
	// Create a temporary directory for the dump file
	tempDir, err := os.MkdirTemp("", "pg-migration-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir) // Clean up temp directory when done

	timestamp := time.Now().Format("20060102-150405")
	dumpFilePath := filepath.Join(tempDir, fmt.Sprintf("schema-dump-%s.sql", timestamp))

	// Dump schema from source with clean options
	if err := s.dumpSchema(dumpFilePath); err != nil {
		return fmt.Errorf("failed to dump schema: %v", err)
	}
	log.Printf("Schema dumped successfully to: %s\n", dumpFilePath)

	// First drop existing objects in target database
	if err := s.dropExistingObjects(); err != nil {
		return fmt.Errorf("failed to drop existing objects: %v", err)
	}

	// Restore schema to target
	if err := s.restoreSchema(dumpFilePath); err != nil {
		return fmt.Errorf("failed to restore schema: %v", err)
	}
	log.Printf("Schema restored successfully to target database\n")

	return nil
}

// dropExistingObjects drops all existing objects in the target database
func (s *SchemaHandler) dropExistingObjects() error {
	// Create a script to drop all existing objects
	dropScript := `
DO $$ 
DECLARE
    schema_rec RECORD;
BEGIN
    -- Drop all schemas except public and pg_* schemas
    FOR schema_rec IN (
        SELECT schema_name 
        FROM information_schema.schemata 
        WHERE schema_name NOT LIKE 'pg_%' 
        AND schema_name != 'public'
        AND schema_name != 'information_schema'
    ) LOOP
        EXECUTE 'DROP SCHEMA IF EXISTS ' || quote_ident(schema_rec.schema_name) || ' CASCADE';
    END LOOP;
    
    -- Drop all functions in public schema
    FOR schema_rec IN (
        SELECT proname, oidvectortypes(proargtypes) AS argtypes
        FROM pg_proc
        INNER JOIN pg_namespace ON pg_proc.pronamespace = pg_namespace.oid
        WHERE nspname = 'public'
    ) LOOP
        EXECUTE 'DROP FUNCTION IF EXISTS public.' || quote_ident(schema_rec.proname) || '(' || schema_rec.argtypes || ') CASCADE';
    END LOOP;
    
    -- Drop all tables, types and other objects in public schema
    FOR schema_rec IN (
        SELECT tablename 
        FROM pg_tables 
        WHERE schemaname = 'public'
    ) LOOP
        EXECUTE 'DROP TABLE IF EXISTS public.' || quote_ident(schema_rec.tablename) || ' CASCADE';
    END LOOP;
    
    -- Drop all types in public schema
    FOR schema_rec IN (
        SELECT typname 
        FROM pg_type
        INNER JOIN pg_namespace ON pg_type.typnamespace = pg_namespace.oid
        WHERE nspname = 'public'
        AND typtype = 'c'  -- Composite types
    ) LOOP
        EXECUTE 'DROP TYPE IF EXISTS public.' || quote_ident(schema_rec.typname) || ' CASCADE';
    END LOOP;
    
    -- Drop all sequences in public schema
    FOR schema_rec IN (
        SELECT relname 
        FROM pg_class
        INNER JOIN pg_namespace ON pg_class.relnamespace = pg_namespace.oid
        WHERE nspname = 'public'
        AND relkind = 'S'  -- Sequences
    ) LOOP
        EXECUTE 'DROP SEQUENCE IF EXISTS public.' || quote_ident(schema_rec.relname) || ' CASCADE';
    END LOOP;
    
    -- Drop all views in public schema
    FOR schema_rec IN (
        SELECT viewname 
        FROM pg_views 
        WHERE schemaname = 'public'
    ) LOOP
        EXECUTE 'DROP VIEW IF EXISTS public.' || quote_ident(schema_rec.viewname) || ' CASCADE';
    END LOOP;
END $$;
`

	// Create a temporary file for the drop script
	tempFile, err := os.CreateTemp("", "drop-objects-*.sql")
	if err != nil {
		return fmt.Errorf("failed to create temp file for drop script: %v", err)
	}
	defer os.Remove(tempFile.Name())

	_, err = tempFile.WriteString(dropScript)
	if err != nil {
		return fmt.Errorf("failed to write drop script to temp file: %v", err)
	}
	tempFile.Close()

	// Execute the drop script using psql
	args := []string{
		"-h", s.target.Host,
		"-p", fmt.Sprintf("%d", s.target.Port),
		"-U", s.target.User,
		"-d", s.target.Database,
		"-f", tempFile.Name(),
	}

	cmd := exec.Command("psql", args...)

	// Set the PGPASSWORD environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.target.Password))

	// Set the PGSSLMODE environment variable if SSL mode is specified
	if s.target.SSLMode != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("PGSSLMODE=%s", s.target.SSLMode))
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("dropping objects failed: %v, stderr: %s", err, stderr.String())
	}

	return nil
}

// DumpSchemaToFile dumps the schema to a specified file path
func (s *SchemaHandler) DumpSchemaToFile(filePath string) error {
	return s.dumpSchema(filePath)
}

// RestoreSchemaFromFile restores the schema from a specified file path
func (s *SchemaHandler) RestoreSchemaFromFile(filePath string) error {
	// First drop existing objects
	if err := s.dropExistingObjects(); err != nil {
		return fmt.Errorf("failed to drop existing objects: %v", err)
	}

	return s.restoreSchema(filePath)
}

// dumpSchema dumps the schema from the source database to a file
func (s *SchemaHandler) dumpSchema(dumpFilePath string) error {
	// pg_dump command with schema-only option
	args := []string{
		"-h", s.source.Host,
		"-p", fmt.Sprintf("%d", s.source.Port),
		"-U", s.source.User,
		"-d", s.source.Database,
		"--schema-only",   // Only dump the schema, not the data
		"--no-owner",      // Don't output commands to set ownership
		"--no-privileges", // Don't output privileges (GRANT/REVOKE)
		"-f", dumpFilePath,
	}

	cmd := exec.Command("pg_dump", args...)

	// Set the PGPASSWORD environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.source.Password))

	// Set the PGSSLMODE environment variable if SSL mode is specified
	if s.source.SSLMode != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("PGSSLMODE=%s", s.source.SSLMode))
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %v, stderr: %s", err, stderr.String())
	}

	return nil
}

// restoreSchema restores the schema to the target database from a file
func (s *SchemaHandler) restoreSchema(dumpFilePath string) error {
	// First, read the file to handle it as stdin for psql
	file, err := os.Open(dumpFilePath)
	if err != nil {
		return fmt.Errorf("failed to open dump file: %v", err)
	}
	defer file.Close()

	// Modify dump file to add IF NOT EXISTS to CREATE SCHEMA statements
	// and OR REPLACE to function definitions
	modifiedDumpPath := dumpFilePath + ".modified"
	modifiedFile, err := os.Create(modifiedDumpPath)
	if err != nil {
		return fmt.Errorf("failed to create modified dump file: %v", err)
	}
	defer os.Remove(modifiedDumpPath) // Clean up the modified file afterwards
	defer modifiedFile.Close()

	// Process the dump file
	err = processSchemaFile(file, modifiedFile)
	if err != nil {
		return fmt.Errorf("failed to process schema file: %v", err)
	}

	// Reopen the modified file for reading
	modifiedFile.Close()
	modifiedFile, err = os.Open(modifiedDumpPath)
	if err != nil {
		return fmt.Errorf("failed to open modified dump file: %v", err)
	}
	defer modifiedFile.Close()

	// psql command to restore
	args := []string{
		"-h", s.target.Host,
		"-p", fmt.Sprintf("%d", s.target.Port),
		"-U", s.target.User,
		"-d", s.target.Database,
		"-v", "ON_ERROR_STOP=1", // Stop execution if there's an error
	}

	cmd := exec.Command("psql", args...)
	cmd.Stdin = modifiedFile // Use the modified dump file as input

	// Set the PGPASSWORD environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.target.Password))

	// Set the PGSSLMODE environment variable if SSL mode is specified
	if s.target.SSLMode != "" {
		cmd.Env = append(cmd.Env, fmt.Sprintf("PGSSLMODE=%s", s.target.SSLMode))
	}

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("psql restore failed: %v, stderr: %s", err, stderr.String())
	}

	return nil
}

// processSchemaFile modifies SQL statements to make them idempotent
func processSchemaFile(input io.Reader, output io.Writer) error {
	// Read the entire file
	content, err := io.ReadAll(input)
	if err != nil {
		return fmt.Errorf("failed to read input file: %v", err)
	}

	// Convert to string for easier processing
	sqlContent := string(content)

	// 1. Replace CREATE SCHEMA with CREATE SCHEMA IF NOT EXISTS
	sqlContent = strings.Replace(
		sqlContent,
		"CREATE SCHEMA ",
		"CREATE SCHEMA IF NOT EXISTS ",
		-1,
	)

	// 2. Replace CREATE FUNCTION with CREATE OR REPLACE FUNCTION
	sqlContent = strings.Replace(
		sqlContent,
		"CREATE FUNCTION ",
		"CREATE OR REPLACE FUNCTION ",
		-1,
	)

	// 3. Replace CREATE TABLE with CREATE TABLE IF NOT EXISTS
	sqlContent = strings.Replace(
		sqlContent,
		"CREATE TABLE ",
		"CREATE TABLE IF NOT EXISTS ",
		-1,
	)

	// 4. Replace CREATE INDEX with CREATE INDEX IF NOT EXISTS
	sqlContent = strings.Replace(
		sqlContent,
		"CREATE INDEX ",
		"CREATE INDEX IF NOT EXISTS ",
		-1,
	)

	// 5. Replace CREATE SEQUENCE with CREATE SEQUENCE IF NOT EXISTS
	sqlContent = strings.Replace(
		sqlContent,
		"CREATE SEQUENCE ",
		"CREATE SEQUENCE IF NOT EXISTS ",
		-1,
	)

	// 6. Replace CREATE VIEW with CREATE OR REPLACE VIEW
	sqlContent = strings.Replace(
		sqlContent,
		"CREATE VIEW ",
		"CREATE OR REPLACE VIEW ",
		-1,
	)

	// Write the modified content to the output
	_, err = output.Write([]byte(sqlContent))
	if err != nil {
		return fmt.Errorf("failed to write to output file: %v", err)
	}

	return nil
}

// DumpSchemaToWriter dumps the schema from the source database to a writer
func (s *SchemaHandler) DumpSchemaToWriter(writer io.Writer) error {
	// pg_dump command with schema-only option
	args := []string{
		"-h", s.source.Host,
		"-p", fmt.Sprintf("%d", s.source.Port),
		"-U", s.source.User,
		"-d", s.source.Database,
		"--schema-only",   // Only dump the schema, not the data
		"--no-owner",      // Don't output commands to set ownership
		"--no-privileges", // Don't output privileges (GRANT/REVOKE)
	}

	// Add SSL mode if specified
	if s.source.SSLMode != "" {
		args = append(args, "--sslmode="+s.source.SSLMode)
	}

	cmd := exec.Command("pg_dump", args...)

	// Set the PGPASSWORD environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.source.Password))

	cmd.Stdout = writer
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("pg_dump failed: %v, stderr: %s", err, stderr.String())
	}

	return nil
}

// RestoreSchemaFromReader restores the schema to the target database from a reader
func (s *SchemaHandler) RestoreSchemaFromReader(reader io.Reader) error {
	// First drop existing objects
	if err := s.dropExistingObjects(); err != nil {
		return fmt.Errorf("failed to drop existing objects: %v", err)
	}

	// Create a buffer to hold the reader contents
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return fmt.Errorf("failed to read input: %v", err)
	}

	// Modify the content
	content := buf.String()

	// Same modifications as in processSchemaFile
	content = strings.Replace(content, "CREATE SCHEMA ", "CREATE SCHEMA IF NOT EXISTS ", -1)
	content = strings.Replace(content, "CREATE FUNCTION ", "CREATE OR REPLACE FUNCTION ", -1)
	content = strings.Replace(content, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ", -1)
	content = strings.Replace(content, "CREATE INDEX ", "CREATE INDEX IF NOT EXISTS ", -1)
	content = strings.Replace(content, "CREATE SEQUENCE ", "CREATE SEQUENCE IF NOT EXISTS ", -1)
	content = strings.Replace(content, "CREATE VIEW ", "CREATE OR REPLACE VIEW ", -1)

	// psql command to restore
	args := []string{
		"-h", s.target.Host,
		"-p", fmt.Sprintf("%d", s.target.Port),
		"-U", s.target.User,
		"-d", s.target.Database,
		"-v", "ON_ERROR_STOP=1", // Stop execution if there's an error
	}

	// Add SSL mode if specified
	if s.target.SSLMode != "" {
		args = append(args, fmt.Sprintf("--sslmode=%s", s.target.SSLMode))
	}

	cmd := exec.Command("psql", args...)
	cmd.Stdin = strings.NewReader(content) // Use the modified content as input

	// Set the PGPASSWORD environment variable
	cmd.Env = append(os.Environ(), fmt.Sprintf("PGPASSWORD=%s", s.target.Password))

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("psql restore failed: %v, stderr: %s", err, stderr.String())
	}

	return nil
}
