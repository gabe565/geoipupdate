// geoipupdate performs automatic updates of GeoIP binary databases.
package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/maxmind/geoipupdate/v4/pkg/geoipupdate"
	"github.com/maxmind/geoipupdate/v4/pkg/geoipupdate/database"
)

var (
	version                  = "unknown"
	defaultConfigFile        string
	defaultDatabaseDirectory string
)

func main() {
	log.SetFlags(0)

	if defaultConfigFile == "" {
		defaultConfigFile = geoipupdate.DefaultConfigFile
	}
	if defaultDatabaseDirectory == "" {
		defaultDatabaseDirectory = geoipupdate.DefaultDatabaseDirectory
	}

	args := getArgs()
	fatalLogger := func(message string, err error) {
		if args.StackTrace {
			log.Printf("%s: %+v", message, err)
		} else {
			log.Printf("%s: %s", message, err)
		}
		os.Exit(1)
	}

	config, err := geoipupdate.NewConfig(
		args.ConfigFile,
		defaultDatabaseDirectory,
		args.DatabaseDirectory,
		args.Verbose,
		geoipupdate.WithParallelism(args.Parallelism),
	)
	if err != nil {
		fatalLogger(fmt.Sprintf("error loading configuration file %s", args.ConfigFile), err)
	}

	if config.Verbose {
		log.Printf("geoipupdate version %s", version)
		log.Printf("Using config file %s", args.ConfigFile)
		log.Printf("Using database directory %s", config.DatabaseDirectory)
	}

	client := geoipupdate.NewClient(config)

	if err = run(client, config); err != nil {
		fatalLogger("error retrieving updates", err)
	}
}

func run(client *http.Client, config *geoipupdate.Config) error {
	dbReader := database.NewHTTPDatabaseReader(client, config)
	fileLock, err := database.NewFileLock(config.LockFile, config.Verbose)
	if err != nil {
		return fmt.Errorf("error initializing file lock: %w", err)
	}
	defer func() {
		if err := fileLock.Close(); err != nil {
			log.Printf("error closing file lock: %s", err)
		}
	}()

	for _, editionID := range config.EditionIDs {
		filename, err := geoipupdate.GetFilename(config, editionID, client)
		if err != nil {
			return fmt.Errorf("error retrieving filename for %s: %w", editionID, err)
		}
		filePath := filepath.Join(config.DatabaseDirectory, filename)
		dbWriter, err := database.NewLocalFileDatabaseWriter(filePath, fileLock, config.Verbose)
		if err != nil {
			return fmt.Errorf("error creating database writer for %s: %w", editionID, err)
		}
		dbReader.Queue(dbWriter, editionID)
	}

	if err := dbReader.Wait(); err != nil {
		return fmt.Errorf("download error: %w", err)
	}
	return nil
}
