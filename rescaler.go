package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog/log"

	"github.com/aleksasiriski/rffmpeg-autoscaler/processor"
	"github.com/google/uuid"
	"github.com/hetznercloud/hcloud-go/hcloud"
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
							err := createWorker(config, proc, client)
							if err != nil {
								log.Error().
									Err(err).
									Msg("Failed to create a new worker")
							}
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
					err := createWorker(config, proc, client)
					if err != nil {
						log.Error().
							Err(err).
							Msg("Failed to create a new worker")
					}
				}
			}
		}
	}

	log.Info().
		Msg("Finished checking for processes and rescaling.")
}

func createWorker(config Config, proc *processor.Processor, client *hcloud.Client) error {
	log.Info().
		Msg("Creating a new worker.")

	log.Debug().
		Msg("Sending request to get objects from Hetzner.")

	ctx := context.Background()

	name := generateName(ctx, client)
	serverType, _, err := client.ServerType.GetByName(ctx, config.Hetzner.Server)
	image, _, err := client.Image.GetByName(ctx, config.Hetzner.Image)
	sshKey, _, err := client.SSHKey.GetByName(ctx, config.Hetzner.SshKey)
	location, _, err := client.Location.GetByName(ctx, config.Hetzner.Location)
	network, _, err := client.Network.GetByName(ctx, config.Hetzner.Network)
	firewall, _, err := client.Firewall.GetByName(ctx, config.Hetzner.Firewall)
	placementGroup, _, err := client.PlacementGroup.GetByName(ctx, config.Hetzner.PlacementGroup)

	sshKeys := []*hcloud.SSHKey{sshKey}
	networks := []*hcloud.Network{network}
	firewalls := []*hcloud.ServerCreateFirewall{&hcloud.ServerCreateFirewall{
		Firewall: *firewall,
	}}

	log.Debug().
		Msg("Sending request to create worker on Hetzner.")

	serverCreateResult, _, err := client.Server.Create(ctx, hcloud.ServerCreateOpts{
		Name:           name,
		ServerType:     serverType,
		Image:          image,
		SSHKeys:        sshKeys,
		Location:       location,
		UserData:       config.Hetzner.CloudInit,
		Networks:       networks,
		Firewalls:      firewalls,
		PlacementGroup: placementGroup,
	})
	if err != nil {
		return err
	}

	serverName := serverCreateResult.Server.Name
	server, _, err := client.Server.GetByName(ctx, serverName)
	if err != nil {
		return err
	}

	for server.Status != hcloud.ServerStatusRunning {
		server, _, err = client.Server.GetByName(ctx, serverName)
		if err != nil {
			return err
		}
		log.Debug().
			Msg(fmt.Sprintf("Waiting for worker to become available: %s", server.Status))
		time.Sleep(time.Second * 5)
	}

	serverIp := server.PrivateNet[0].IP.String()

	log.Info().
		Msg(fmt.Sprintf("Created a new worker %s.", serverName))

	host := processor.Host{
		Servername: serverName,
		Hostname:   serverIp,
		Weight:     config.Jellyfin.Weight,
		Created:    time.Now(),
	}

	log.Debug().
		Msg(fmt.Sprintf("Adding worker %s with hostname %s to database.", serverName, serverIp))

	err = proc.AddHosts(host)

	if err != nil {
		log.Error().
			Err(err).
			Msg(fmt.Sprintf("Failed adding worker %s to database.", serverName))
	}
	log.Debug().
		Msg(fmt.Sprintf("Added worker %s to database.", serverName))

	return err
}

func generateName(ctx context.Context, client *hcloud.Client) string {
	servers, _ := client.Server.All(ctx)

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
			return name
		}
	}
}
