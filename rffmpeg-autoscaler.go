package main

import (
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
	//"strings"

	"github.com/alecthomas/kong"
	"gopkg.in/natefinch/lumberjack.v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sourcegraph/conc"

	"database/sql"
	_ "modernc.org/sqlite"
	_ "github.com/lib/pq"

	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/aleksasiriski/rffmpeg-autoscaler/migrate"
	"github.com/aleksasiriski/rffmpeg-autoscaler/processor"
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
			"config_dir":	"/config",
			"log_file":		"/config/log/rffmpeg-autoscaler.log",
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
	db, err := sql.Open(config.Database.Type, config.Database.Path)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed opening datastore:")
	}
	if config.Database.Type == "sqlite" {
		db.SetMaxOpenConns(1)
	}

	// migrator
	mg, err := migrate.New(db, config.Database.Type, config.Database.MigratorDir)
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed initialising migrator:")
	}

	// processor
	proc, err := processor.New(processor.Config{
		Db:         db,
		DbType:     config.Database.Type,
		Mg:         mg,
	})
	if err != nil {
		log.Fatal().
			Err(err).
			Msg("Failed initialising processor:")
	}

	// hetzner
	client := hcloud.NewClient(hcloud.WithToken(config.Hetzner.Token))

	// display initialised banner
	log.Info().
		Str("Migrator", fmt.Sprintf("success")).
		Str("Processor", fmt.Sprintf("success")).
		Str("Hcloud", fmt.Sprintf("success")).
		Msg("Initialised")

	// rffmpeg-autoscaler
	exit := false
	var wg conc.WaitGroup
	wg.Go(func () {
		CheckProcessesAndRescale(config, proc, client, &exit)
	})

	// handle interrupt signal
	quitChannel := make(chan os.Signal, 1)
    signal.Notify(quitChannel, syscall.SIGINT, syscall.SIGTERM)
	
	// cleanup on interrupt signal
	<-quitChannel
	exit = true
	wg.Wait()
}

func CheckProcessesAndRescale(config Config, proc *processor.Processor, client *hcloud.Client, exit *bool) {
    log.Info().
		Msg("Started checking for processes and rescaling")
	for !*exit {
		numberOfHosts, err := proc.NumberOfHosts()
		if err != nil {
			log.Error().
				Err(err).
				Msg("Failed getting the current number of hosts:")
		} else {
			if numberOfHosts == 0 {
				log.Debug().
					Msg("No workers found. Checking if there are any transcodes on fallback.")
				transcodes := 0
				numberOfProcesses, err := proc.NumberOfProcesses()
				if err != nil {
					log.Error().
						Err(err).
						Msg("Failed getting the current number of processes:")
				} else {
					if numberOfProcesses == 0 {
						log.Debug().
							Msg("Found no processes on fallback.")
					} else {
						processes, err := proc.GetAllProcesses()
						if err != nil {
							log.Error().
								Err(err).
								Msg("Failed getting all processes:")
						} else {
							for _, process := range processes {
								/*if strings.Contains(process.cmd, "transcode") {
									transcodes += 1
								}*/
								fmt.Printf(fmt.Sprintf("%s", process))
							}
							if transcodes > 0 {
								log.Info().
									Msg(fmt.Sprintf("Found %d transcodes on fallback.", transcodes))
								//createServer
							} else {
								log.Debug().
									Msg("Found no transcodes on fallback.")
							}
								
						}
					}
				}
			}
		}
		time.Sleep(time.Second * 5) // Should be Minute
    }
	log.Info().
		Msg("Finished checking for processes and rescaling")
}