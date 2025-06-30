package databasedetectionstage

import (
	"fmt"
	"regexp"
	"strings"
)

// DatabaseType defines a custom type for database types.
type DatabaseType string

// Enum-like constants for known database types.
const (
	MySQL      DatabaseType = "MySQL"
	PostgreSQL DatabaseType = "PostgreSQL"
	MongoDB    DatabaseType = "MongoDB"
	Redis      DatabaseType = "Redis"
	Cassandra  DatabaseType = "Cassandra"
	DynamoDB   DatabaseType = "DynamoDB"
	SQLite     DatabaseType = "SQLite"
	SQLServer  DatabaseType = "SQLServer"
	CosmosDB   DatabaseType = "CosmosDB"
)

// KnownDatabaseTypes is a list of all valid database types.
var KnownDatabaseTypes = []DatabaseType{
	MySQL,
	PostgreSQL,
	MongoDB,
	Redis,
	Cassandra,
	DynamoDB,
	SQLite,
	SQLServer,
	CosmosDB,
}

// DatabasePatterns maps each supported database type to a regex pattern for detecting its name or alias.
var DatabasePatterns map[DatabaseType]*regexp.Regexp

// VersionPatterns maps each supported database type to a regex pattern for extracting its version.
var VersionPatterns map[DatabaseType]*regexp.Regexp

func init() {
	// Define database name aliases for each type.
	dbAliases := map[DatabaseType][]string{
		MySQL:      {"mysql", "mariadb"},
		PostgreSQL: {"postgresql", "postgres"},
		MongoDB:    {"mongodb"},
		Redis:      {"redis"},
		Cassandra:  {"cassandra"},
		DynamoDB:   {"dynamodb"},
		SQLite:     {"sqlite"},
		SQLServer:  {"sqlserver", "mssql"},
		CosmosDB:   {"cosmosdb"},
	}

	// Initialize the maps.
	DatabasePatterns = make(map[DatabaseType]*regexp.Regexp)
	VersionPatterns = make(map[DatabaseType]*regexp.Regexp)

	// Compile the patterns for each database type.
	for dbType, aliases := range dbAliases {
		dbPattern, err := DatabasePattern(aliases)
		if err != nil {
			panic(fmt.Sprintf("Failed to compile database pattern for %s: %v", dbType, err))
		}
		DatabasePatterns[dbType] = dbPattern

		versionPattern, err := VersionPattern(aliases)
		if err != nil {
			panic(fmt.Sprintf("Failed to compile version pattern for %s: %v", dbType, err))
		}
		VersionPatterns[dbType] = versionPattern
	}
}

// DatabasePattern generates a regex pattern to match database names or aliases.
func DatabasePattern(dbStrings []string) (*regexp.Regexp, error) {
	// Join the database strings into a regex pattern with alternation (|) and word boundaries (\b).
	// The regex patterns are case-insensitive and designed to match common database names or aliases.
	pattern := fmt.Sprintf(`(?i)\b%s\b`, strings.Join(dbStrings, `\b|(?i)\b`))
	return regexp.Compile(pattern)
}

// VersionPattern generates a regex pattern to extract version numbers for a database.
func VersionPattern(dbStrings []string) (*regexp.Regexp, error) {
	// Join the database strings into a regex pattern with alternation (|).
	// versionPatterns maps each supported database type to a corresponding regular expression
	// The regex patterns are case-insensitive and designed to match various common formats:
	//   - Plain text format: "mysql 8.0.23", "postgresql-12.3"
	//   - XML/markup format: "<mysql.version>8.0.23</mysql.version>"
	//   - Key-value format: "mysql.version 8.0.23"
	// Each pattern captures version numbers in the form of "X.Y" or "X.Y.Z".
	aliases := strings.Join(dbStrings, "|")
	pattern := fmt.Sprintf(`(?i)(%s)(?:[\s-]?version)?[\s-]?(\d+\.\d+(\.\d+)?)|<(%s)\.version>(\d+\.\d+(\.\d+)?)</(%s)\.version>|(%s)\.version[\s-]?(\d+\.\d+(\.\d+)?)`,
		aliases, aliases, aliases, aliases)
	return regexp.Compile(pattern)
}
