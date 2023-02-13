package processor

import (
	"database/sql"
	"time"

	"github.com/aleksasiriski/rffmpeg-autoscaler/migrate"
)

type Config struct {
	Db *sql.DB
	DbType string
	Mg *migrate.Migrator
}

type Host struct {
	id int
	servername string
	hostname string
	weight int
	created time.Time
}

type Process struct {
	id int
	host_id int
	process_id int
	cmd string
}

type State struct {
	id int
	host_id int
	process_id int
	state string
}

func New(config Config) (*Processor, error) {
	store, err := newDatastore(config.Db, config.DbType, config.Mg)
	if err != nil {
		return nil, err
	}

	proc := &Processor{
		store: store,
	}
	return proc, nil
}

type Processor struct {
	store      *datastore
	processed  int64
}

func (p *Processor) AddHosts(hosts ...Host) error {
	return p.store.UpsertHosts(hosts)
}

func (p *Processor) RemoveHost(host Host) error {
	return p.store.DeleteHost(host)
}

func (p *Processor) NumberOfHosts() (int, error) {
	return p.store.GetHostsRemaining()
}

func (p *Processor) GetAllHosts() ([]Host, error) {
	return p.store.GetHosts()
}

func (p *Processor) NumberOfProcesses() (int, error) {
	return p.store.GetProcessesRemaining()
}

func (p *Processor) GetAllProcesses() ([]Process, error) {
	return p.store.GetProcesses()
}

func (p *Processor) GetAllProcessesFromHost(host Host) ([]Process, error) {
	return p.store.GetProcessesWhere(host)
}

func (p *Processor) GetAllStatesFromHost(host Host) ([]State, error) {
	return p.store.GetStatesWhere(host)
}