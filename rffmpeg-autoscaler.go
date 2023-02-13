package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"database/sql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/sourcegraph/conc"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/aleksasiriski/rffmpeg-autoscaler/migrate"
	"github.com/aleksasiriski/rffmpeg-autoscaler/processor"
	"github.com/hetznercloud/hcloud-go/hcloud"
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
			"config_dir": "/config",
			"log_file":   "/config/log/rffmpeg-autoscaler.log",
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
		Db:     db,
		DbType: config.Database.Type,
		Mg:     mg,
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
	needToExit := false
	ableToExit := false
	var wg conc.WaitGroup
	wg.Go(func() {
		CheckProcessesAndRescale(config, proc, client, &needToExit, &ableToExit)
	})

	// handle interrupt signal
	quitChannel := make(chan os.Signal, 1)
	signal.Notify(quitChannel, syscall.SIGINT, syscall.SIGTERM)

	// cleanup on interrupt signal
	<-quitChannel
	needToExit = true
	if !ableToExit {
		wg.Wait()
	}
}

func CheckProcessesAndRescale(config Config, proc *processor.Processor, client *hcloud.Client, needToExit *bool, ableToExit *bool) {
	log.Info().
		Msg("Started checking for processes and rescaling")

	for !*needToExit {
		*ableToExit = false

		numberOfHosts, err := proc.NumberOfHosts()
		if err != nil {
			log.Error().
				Err(err).
				Msg("Failed getting the current number of hosts:")
		} else {
			if numberOfHosts == 0 {
				log.Info().
					Msg("No workers found. Checking if there are any transcodes on fallback.")
				transcodes := 0
				numberOfProcesses, err := proc.NumberOfProcesses()
				if err != nil {
					log.Error().
						Err(err).
						Msg("Failed getting the current number of processes:")
				} else {
					if numberOfProcesses == 0 {
						log.Info().
							Msg("Found no processes on fallback.")
					} else {
						processes, err := proc.GetAllProcesses()
						if err != nil {
							log.Error().
								Err(err).
								Msg("Failed getting all processes:")
						} else {
							for _, process := range processes {
								if strings.Contains(process.Cmd, "transcode") {
									transcodes += 1
								}
							}
							if transcodes > 0 {
								log.Info().
									Msg(fmt.Sprintf("Found %d transcodes on fallback.", transcodes))
								//! createServer
							} else {
								log.Info().
									Msg("Found no transcodes on fallback.")
							}
						}
					}
				}
			} else {
				log.Info().
					Msg("Workers found. Checking if there are any workers with room.")
				workers_with_room := 0
				hosts, err := proc.GetAllHosts()
				if err != nil {
					log.Error().
						Err(err).
						Msg("Failed getting all hosts:")
				} else {
					for _, host := range hosts {
						transcodes := 0
						numberOfProcesses, err := proc.NumberOfProcessesFromHost(host)
						if err != nil {
							log.Error().
								Err(err).
								Msg(fmt.Sprintf("Failed getting the current number of processes for host %s:", host.Servername))
						} else {
							if numberOfProcesses == 0 {
								log.Info().
									Msg(fmt.Sprintf("Found no processes on host %s.", host.Servername))
							} else {
								processes, err := proc.GetAllProcessesFromHost(host)
								if err != nil {
									log.Error().
										Err(err).
										Msg(fmt.Sprintf("Failed getting all processes from host %s:", host.Servername))
								} else {
									for _, process := range processes {
										if strings.Contains(process.Cmd, "transcode") {
											transcodes += 1
										}
									}
									if transcodes > config.Jellyfin.Jobs {
										log.Info().
											Msg(fmt.Sprintf("Found %d transcodes on host %s.", transcodes, host.Servername))

									} else {
										log.Info().
											Msg(fmt.Sprintf("Found less than %d transcodes on host %s.", config.Jellyfin.Jobs, host.Servername))
										workers_with_room += 1
									}
								}
							}
						}
					}
					if workers_with_room > 0 {
						log.Info().
							Msg(fmt.Sprintf("Found %d workers with room.", workers_with_room))
					} else {
						log.Info().
							Msg("All workers are busy, creating a new one.")
						//! createServer
					}
				}
			}
		}

		*ableToExit = true
		log.Info().
			Msg("Sleeping for 5 minutes until next check.")
		time.Sleep(time.Minute * 5)
	}

	log.Info().
		Msg("Finished checking for processes and rescaling")
}

func createServer(config Config, proc *processor.Processor, client *hcloud.Client, needToExit *bool, ableToExit *bool) error {
	ctx := context.Background()

	//! name := generateName()
	serverType, _, err := client.ServerType.GetByName(ctx, config.Hetzner.Server)
	image, _, err := client.Image.GetByName(ctx, config.Hetzner.Image)
	sshKey, _, err := client.SSHKey.GetByName(ctx, config.Hetzner.SshKey)
	location, _, err := client.Location.GetByName(ctx, config.Hetzner.Location)
	network, _, err := client.Network.GetByName(ctx, config.Hetzner.Network)
	//firewall, _, err := client.Firewall.GetByName(ctx, config.Hetzner.Firewall)
	placementGroup, _, err := client.PlacementGroup.GetByName(ctx, config.Hetzner.PlacementGroup)

	sshKeys := []*hcloud.SSHKey{sshKey}
	networks := []*hcloud.Network{network}
	/*firewalls := []*hcloud.ServerCreateFirewall{*hcloud.ServerCreateFirewall{
		Firewall: *firewall,
	}}*/

	server, _, err := client.Server.Create(ctx, hcloud.ServerCreateOpts{
		//! Name: name,
		ServerType: serverType,
		Image:      image,
		SSHKeys:    sshKeys,
		Location:   location,
		UserData:   config.Hetzner.CloudInit,
		Networks:   networks,
		//!Firewalls: firewalls,
		PlacementGroup: placementGroup,
	})

	fmt.Printf("%s", server)

	return err
}
