package main

import (
	"fmt"
	"strings"
	"github.com/spf13/viper"
)

type Jellyfin struct {
    Host	string 	`mapstructure:"HOST"`
	SshKey	string 	`mapstructure:"SSH_KEY"`
    Jobs	int 	`mapstructure:"JOBS"`
}

type Hetzner struct {
    Token			string	`mapstructure:"TOKEN"`
    Server			string	`mapstructure:"SERVER"`
    Image			string	`mapstructure:"IMAGE"`
	SshKey			string	`mapstructure:"SSH_KEY"`
	Network			string	`mapstructure:"NETWORK"`
	Firewall		string	`mapstructure:"FIREWALL"`
	PlacementGroup	string	`mapstructure:"PLACEMENT_GROUP"`
	Location		string	`mapstructure:"LOCATION"`
	CloudInit		string	`mapstructure:"CLOUD_INIT"`
}

type Database struct {
	Type		string	`mapstructure:"type"`
	Host		string	`mapstructure:"host"`
	Port		int		`mapstructure:"port"`
	Name		string	`mapstructure:"name"`
	Username	string	`mapstructure:"username"`
	Password	string	`mapstructure:"password"`
}

type Media struct {
    Username	string `mapstructure:"USERNAME"`
    Password	string `mapstructure:"PASSWORD"`
}

type Config struct {
	Jellyfin	Jellyfin	`mapstructure:"JELLYFIN"`
    Hetzner		Hetzner		`mapstructure:"HETZNER"`
	Database	Database	`mapstructure:"DATABASE"`
	Media		Media		`mapstructure:"MEDIA"`
}

func LoadConfig(path string) (config Config, err error) {
	config = Config{
		Jellyfin: Jellyfin{
			SshKey: "/config/rffmpeg/.ssh/id_ed25519.pub"
			Jobs: 2,
		},
		Hetzner: Hetzner{
			Server: "cpx21",
			Image: "docker-ce",
			SshKey: "root@jellyfin",
			Network: "rffmpeg-workers",
			Firewall: "rffmpeg-workers",
			PlacementGroup: "rffmpeg-workers",
			Location: "nbg1",
			CloudInit: "#cloud-config\nruncmd:\n- systemctl disable --now ssh.service\n- echo 'JELLYFIN_LAN_ONLY_IP=%s' | tee -a /root/.env\n- echo 'MEDIA_USERNAME=%s' | tee -a /root/.env\n- echo 'MEDIA_PASSWORD=%s' | tee -a /root/.env\n- wget https://raw.githubusercontent.com/aleksasiriski/rffmpeg-worker/main/docker-compose.example.yml -O /root/docker-compose.yml\n- cd /root && docker compose pull && docker compose up -d\n",
		},
		Database: Database{
			Type: "sqlite",
			Host: "localhost",
			Port: 5432,
			Name: "rffmpeg-autoscaler",
			Username: "postgres",
		},
	}

	viper.AddConfigPath(path)
	viper.SetConfigName("rffmpeg-autoscaler")
	viper.SetConfigType("yaml")

	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			panic(fmt.Errorf("Failed parsing config: %w", err))
		}
	}

	err = viper.Unmarshal(&config)

	if config.Jellyfin.Host == "" {
		panic(fmt.Errorf("Jellyfin host is not specified!"))
	} else if config.Hetzner.Token == "" {
		panic(fmt.Errorf("Hetzner token is not specified!"))
	} else if config.Database.Type != "sqlite" && config.Database.Type != "postgres" {
		panic(fmt.Errorf("Database type must be sqlite or postgres!"))
	}
	config.Hetzner.CloudInit = fmt.Sprintf(config.Hetzner.CloudInit, config.Jellyfin.Host, config.Media.Username, config.Media.Password)

	return
}