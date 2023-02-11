package main

import (
	"fmt"
	"io"
	"os"
	"time"

	"github.com/alecthomas/kong"
	"github.com/natefinch/lumberjack"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	_ "database/sql"
	_ "modernc.org/sqlite"
	_ "github.com/lib/pq"
)

var (
	// CLI
	cli struct {
		// flags
		Config    string `type:"path" default:"${config_dir}" env:"RFFMPEG_AUTOSCALER_CONFIG" help:"Config file path"`
		Log       string `type:"path" default:"${log_file}" env:"RFFMPEG_AUTOSCALER_LOG" help:"Log file path"`
		Verbosity int    `type:"counter" default:"0" short:"v" env:"RFFMPEG_AUTOSCALER_VERBOSITY" help:"Log level verbosity"`
	}
)

func main() {
	// parse cli
	ctx := kong.Parse(&cli,
		kong.Name("rffmpeg-autoscaler"),
		kong.Description("Autoscale workers for rffmpeg"),
		kong.UsageOnError(),
		kong.ConfigureHelp(kong.HelpOptions{
			Summary: true,
			Compact: true,
		}),
		kong.Vars{
			"config_dir":   "./config",
			"log_file":      "./config/log/rffmpeg-autoscaler.log",
		},
	)

	if err := ctx.Validate(); err != nil {
		fmt.Println("Failed parsing cli:", err)
		os.Exit(1)
	}

	// logger
	logger := log.Output(io.MultiWriter(zerolog.ConsoleWriter{
		TimeFormat: time.Stamp,
		Out:        os.Stderr,
	}, zerolog.ConsoleWriter{
		TimeFormat: time.Stamp,
		Out: &lumberjack.Logger{
			Filename:   cli.Log,
			MaxSize:    5,
			MaxAge:     14,
			MaxBackups: 5,
		},
		NoColor: true,
	}))

	switch {
	case cli.Verbosity == 1:
		log.Logger = logger.Level(zerolog.DebugLevel)
	case cli.Verbosity > 1:
		log.Logger = logger.Level(zerolog.TraceLevel)
	default:
		log.Logger = logger.Level(zerolog.InfoLevel)
	}

	// config
	config, err := LoadConfig(cli.Config)
    if err != nil {
		log.Fatal().
			Err(err).
			Msg("Cannot load config:")
    }

	// datastore
	/*dbconn := cli.Database
	if c.Database.Type == "postgres" {
		dbconn = fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable", c.Database.Host, c.Database.Port, c.Database.Username, c.Database.Password, c.Database.Name)
	} else if c.Database.Type != "sqlite" {
		log.Fatal().
			Msg("Wrong database type")
	}
	db, err := sql.Open(c.Database.Type, dbconn)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed opening datastore")
	}
	db.SetMaxOpenConns(1)

	// migrator
	migratorDir := "migrations/sqlite"
	if c.Database.Type == "postgres" {
		migratorDir = "migrations/postgres"
	}
	mg, err := migrate.New(db, c.Database.Type, migratorDir)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed initialising migrator")
	}*/

	/*log.Info().
		Int("manual", 1).
		Int("bernard", len(c.Triggers.Bernard)).
		Int("inotify", len(c.Triggers.Inotify)).
		Int("lidarr", len(c.Triggers.Lidarr)).
		Int("radarr", len(c.Triggers.Radarr)).
		Int("readarr", len(c.Triggers.Readarr)).
		Int("sonarr", len(c.Triggers.Sonarr)).
		Msg("Initialised triggers")*/

	/*for _, t := range c.Targets.Jellyfin {
		tp, err := jellyfin.New(t)
		if err != nil {
			log.Fatal().
				Err(err).
				Str("target", "jellyfin").
				Str("target_url", t.URL).
				Msg("Failed initialising target")
		}

		targets = append(targets, tp)
	}*/

	// display initialised banner
	log.Info().
		Str("Config", fmt.Sprintf("%s", config)).
		Msg("Initialised")
}
