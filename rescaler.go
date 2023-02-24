package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/aleksasiriski/rffmpeg-go/processor"
	"github.com/google/uuid"
	"github.com/hetznercloud/hcloud-go/hcloud"
	"github.com/sourcegraph/conc"
)

func CheckProcessesAndRescale(config Config, proc *processor.Processor, client *hcloud.Client) {
	log.Info().
		Msg("Started checking for processes and rescaling.")

	numberOfHosts, err := proc.NumberOfHosts()
	if err != nil {
		log.Error().
			Err(err).
			Msg("Failed getting the current number of hosts:")
	} else {
		log.Info().
			Msg("Checking if there are any transcodes on fallback.")

		fallback := processor.Host{
			Id: 0,
		}
		numberOfProcesses, err := proc.NumberOfProcessesFromHost(fallback)
		if err != nil {
			log.Error().
				Err(err).
				Msg("Failed getting the current number of processes:")
		} else {
			fallbackTranscodes := 0
			if numberOfProcesses == 0 {
				log.Info().
					Msg("Found no processes on fallback.")
			} else {
				processes, err := proc.GetAllProcessesFromHost(fallback)
				if err != nil {
					log.Error().
						Err(err).
						Msg("Failed getting all processes:")
				} else {
					for _, process := range processes {
						if strings.Contains(process.Cmd, "transcode") {
							fallbackTranscodes += 1
						}
					}
					if fallbackTranscodes > 0 {
						log.Info().
							Msg(fmt.Sprintf("Found %d transcodes on fallback.", fallbackTranscodes))
					} else {
						log.Info().
							Msg("Found no transcodes on fallback.")
					}
				}
			}

			workersWithRoom := 0
			uselessWorkers := make([]processor.Host, 0)
			if numberOfHosts > 0 {
				log.Info().
					Msg("Workers found. Checking if there are any workers with room.")
				hosts, err := proc.GetAllHosts()
				if err != nil {
					log.Error().
						Err(err).
						Msg("Failed getting all hosts:")
				} else {
					for _, host := range hosts {
						workerTranscodes := 0
						numberOfProcesses, err := proc.NumberOfProcessesFromHost(host)
						if err != nil {
							log.Error().
								Err(err).
								Msg(fmt.Sprintf("Failed getting the current number of processes for host %s:", host.Servername))
						} else {
							if numberOfProcesses == 0 {
								log.Info().
									Msg(fmt.Sprintf("Found no processes on host %s.", host.Servername))
								workersWithRoom += 1
								uselessWorkers = append(uselessWorkers, host)
							} else {
								processes, err := proc.GetAllProcessesFromHost(host)
								if err != nil {
									log.Error().
										Err(err).
										Msg(fmt.Sprintf("Failed getting all processes from host %s:", host.Servername))
								} else {
									for _, process := range processes {
										if strings.Contains(process.Cmd, "transcode") {
											workerTranscodes += 1
										}
									}
									if workerTranscodes > config.Jellyfin.Jobs {
										log.Info().
											Msg(fmt.Sprintf("Found %d transcodes on host %s.", workerTranscodes, host.Servername))
									} else {
										log.Info().
											Msg(fmt.Sprintf("Found less than %d transcodes on host %s.", config.Jellyfin.Jobs, host.Servername))
										workersWithRoom += 1
										if workerTranscodes == 0 {
											uselessWorkers = append(uselessWorkers, host)
										}
									}
								}
							}
						}
					}
					if workersWithRoom > 0 {
						log.Info().
							Msg(fmt.Sprintf("Found %d workers with room.", workersWithRoom))
					} else {
						log.Info().
							Msg("All workers are busy.")
					}
				}
			} else {
				log.Info().
					Msg("Found 0 workers.")
			}

			if fallbackTranscodes > 0 {
				if workersWithRoom == 0 {
					err := createWorker(config, proc, client)
					if err != nil {
						log.Error().
							Err(err).
							Msg("Failed to create a new worker")
					}
				}
			} else {
				var helper conc.WaitGroup
				for _, uselessWorker := range uselessWorkers {
					helper.Go(func() {
						err := deleteWorker(config, proc, client, uselessWorker)
						if err != nil {
							log.Error().
								Err(err).
								Msg(fmt.Sprintf("Failed to delete worker %s", uselessWorker.Servername))
						}
					})
				}
				helper.Wait()
			}
		}
	}

	log.Info().
		Msg("Finished checking for processes and rescaling.")
}

func createWorker(config Config, proc *processor.Processor, client *hcloud.Client) error {
	log.Info().
		Msg("Creating a new worker.")

	ctx := context.Background()

	log.Debug().
		Msg("Sending request to create worker on Hetzner.")

	opts, err := generateServerInfo(ctx, config, client)
	if err != nil {
		return err
	}
	serverCreateResult, _, err := client.Server.Create(ctx, opts)
	if err != nil {
		return err
	}

	Servername := serverCreateResult.Server.Name
	server, _, err := client.Server.GetByName(ctx, Servername)
	if err != nil {
		return err
	}

	for server.Status != hcloud.ServerStatusRunning {
		server, _, err = client.Server.GetByName(ctx, Servername)
		if err != nil {
			return err
		}
		log.Debug().
			Msg(fmt.Sprintf("Waiting for worker to become available: %s", server.Status))
		time.Sleep(time.Second * 5)
	}

	Serverip := server.PrivateNet[0].IP.String()

	log.Debug().
		Msg(fmt.Sprintf("Created a new worker %s on Hetzner.", Servername))

	log.Debug().
		Msg(fmt.Sprintf("Adding worker %s with hostname %s to database.", Servername, Serverip))

	err = proc.AddHost(processor.Host{
		Servername: Servername,
		Hostname:   Serverip,
		Weight:     config.Jellyfin.Weight,
		Created:    time.Now(),
	})
	if err != nil {
		return err
	}

	log.Debug().
		Msg(fmt.Sprintf("Added worker %s to database.", Servername))

	log.Info().
		Msg(fmt.Sprintf("Created a new worker %s.", Servername))

	return err
}

func generateServerInfo(ctx context.Context, config Config, client *hcloud.Client) (hcloud.ServerCreateOpts, error) {
	log.Debug().
		Msg("Sending request to get objects from Hetzner.")

	opts := hcloud.ServerCreateOpts{}

	name, err := generateName(ctx, client)
	if err != nil {
		return opts, err
	}
	serverType, _, err := client.ServerType.GetByName(ctx, config.Hetzner.Server)
	if err != nil {
		return opts, err
	}
	image, _, err := client.Image.GetByName(ctx, config.Hetzner.Image)
	if err != nil {
		return opts, err
	}
	sshKey, _, err := client.SSHKey.GetByName(ctx, config.Hetzner.SshKey)
	if err != nil {
		return opts, err
	}
	location, _, err := client.Location.GetByName(ctx, config.Hetzner.Location)
	if err != nil {
		return opts, err
	}
	network, _, err := client.Network.GetByName(ctx, config.Hetzner.Network)
	if err != nil {
		return opts, err
	}
	firewall, _, err := client.Firewall.GetByName(ctx, config.Hetzner.Firewall)
	if err != nil {
		return opts, err
	}
	placementGroup, _, err := client.PlacementGroup.GetByName(ctx, config.Hetzner.PlacementGroup)
	if err != nil {
		return opts, err
	}

	sshKeys := []*hcloud.SSHKey{sshKey}
	networks := []*hcloud.Network{network}
	firewalls := []*hcloud.ServerCreateFirewall{{
		Firewall: *firewall,
	}}

	opts = hcloud.ServerCreateOpts{
		Name:           name,
		ServerType:     serverType,
		Image:          image,
		SSHKeys:        sshKeys,
		Location:       location,
		UserData:       config.Hetzner.CloudInit,
		Networks:       networks,
		Firewalls:      firewalls,
		PlacementGroup: placementGroup,
	}

	return opts, err
}

func generateName(ctx context.Context, client *hcloud.Client) (string, error) {
	servers, err := client.Server.All(ctx)
	if err != nil {
		return "", err
	}

	for {
		name := "rffmpeg-worker-" + uuid.NewString()
		unused := true
		for _, server := range servers {
			if server.Name == name {
				unused = false
				break
			}
		}
		if unused {
			return name, err
		}
	}
}

func deleteWorker(config Config, proc *processor.Processor, client *hcloud.Client, host processor.Host) error {
	log.Info().
		Msg(fmt.Sprintf("Removing worker %s.", host.Servername))

	log.Debug().
		Msg(fmt.Sprintf("Sending request to get worker %s from Hetzner.", host.Servername))

	ctx := context.Background()

	server, _, err := client.Server.GetByName(ctx, host.Servername)
	if err != nil {
		return err
	}

	log.Debug().
		Msg(fmt.Sprintf("Sending request to remove worker %s from Hetzner.", host.Servername))

	_, err = client.Server.Delete(ctx, server)
	if err != nil {
		return err
	}

	log.Debug().
		Msg(fmt.Sprintf("Removed worker %s from Hetzner.", host.Servername))

	log.Debug().
		Msg(fmt.Sprintf("Removing worker %s from database.", host.Servername))

	err = proc.RemoveHost(host)
	if err != nil {
		return err
	}

	log.Debug().
		Msg(fmt.Sprintf("Removed worker %s from database.", host.Servername))

	return err
}
