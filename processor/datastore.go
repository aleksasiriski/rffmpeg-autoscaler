package processor

import (
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"time"

	"github.com/aleksasiriski/rffmpeg-autoscaler/migrate"

	_ "modernc.org/sqlite"
	_ "github.com/lib/pq"
)

type datastore struct {
	*sql.DB
	DbType string
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

var now = time.Now

var (
	//go:embed migrations/sqlite
	migrationsSqlite embed.FS
)

var (
	//go:embed migrations/postgres
	migrationsPostgres embed.FS
)

func newDatastore(db *sql.DB, dbType string, mg *migrate.Migrator) (*datastore, error) {
	switch dbType {
	case "sqlite": {
		// migrations/sqlite
		if err := mg.Migrate(&migrationsSqlite, "processor"); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	case "postgres": {
		// migrations/postgres
		if err := mg.Migrate(&migrationsPostgres, "processor"); err != nil {
			return nil, fmt.Errorf("migrate: %w", err)
		}
	}
	default:
		panic(fmt.Errorf("Incorrect database type!"))
	}
	return &datastore{db, dbType}, nil
}

func sqlUpsertHost(dbType string) (string) {
	switch dbType {
	case "sqlite":
		return `INSERT INTO hosts (servername, hostname, weight, created)
				VALUES (?, ?, ?, ?)
				ON CONFLICT (servername) DO UPDATE SET
				    hostname = excluded.hostname
				    weight = excluded.weight
				    created = excluded.created
				`
	case "postgres":
		return `INSERT INTO hosts (servername, hostname, weight, created)
				VALUES ($1, $2, $3, $4)
				ON CONFLICT (servername) DO UPDATE SET
				    hostname = excluded.hostname
				    weight = excluded.weight
				    created = excluded.created
				`
	default:
		panic(fmt.Errorf("Incorrect database type!"))
	}
}

func (store *datastore) upsertHost(tx *sql.Tx, dbType string, host Host) error {
	_, err := tx.Exec(sqlUpsertHost(dbType), host.servername, host.hostname, host.weight, host.created)
	return err
}

func (store *datastore) UpsertHost(hosts []Host) error {
	tx, err := store.Begin()
	if err != nil {
		return err
	}

	for _, host := range hosts {
		if err = store.upsertHost(tx, store.DbType, host); err != nil {
			if rollbackErr := tx.Rollback(); rollbackErr != nil {
				panic(rollbackErr)
			}

			return err
		}
	}

	return tx.Commit()
}

func sqlDeleteHost(dbType string) (string) {
	switch dbType {
	case "sqlite":
		return `DELETE FROM hosts WHERE servername=?`
	case "postgres":
		return `DELETE FROM hosts WHERE servername=$1`
	default:
		panic(fmt.Errorf("Incorrect database type!"))
	}
}

func (store *datastore) DeleteHost(host Host) error {
	_, err := store.Exec(sqlDeleteHost(store.DbType), host.servername)
	if err != nil {
		return fmt.Errorf("delete: %w", err)
	}

	return nil
}

const sqlGetHostsRemaining = `SELECT COUNT(servername) FROM hosts`

func (store *datastore) GetHostsRemaining() (int, error) {
	row := store.QueryRow(sqlGetHostsRemaining)

	remaining := 0
	err := row.Scan(&remaining)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return remaining, nil
	case err != nil:
		return remaining, fmt.Errorf("get remaining hosts: %w", err)
	}

	return remaining, nil
}

const sqlGetHosts = `SELECT * FROM hosts ORDER BY created ASC`

func (store *datastore) GetHosts() (hosts []Host, err error) {
	rows, err := store.Query(sqlGetHosts)
	if err != nil {
		return hosts, err
	}

	defer rows.Close()
	for rows.Next() {
		host := Host{}
		err = rows.Scan(&host.id, &host.servername, &host.hostname, &host.weight, &host.created)
		if err != nil {
			return hosts, err
		}

		hosts = append(hosts, host)
	}

	return hosts, rows.Err()
}

const sqlGetProcesses = `SELECT * FROM processes`

func (store *datastore) GetProcesses() (processes []Process, err error) {
	rows, err := store.Query(sqlGetProcesses)
	if err != nil {
		return processes, err
	}

	defer rows.Close()
	for rows.Next() {
		process := Process{}
		err = rows.Scan(&process.id, &process.host_id, &process.process_id, &process.cmd)
		if err != nil {
			return processes, err
		}

		processes = append(processes, process)
	}

	return processes, rows.Err()
}

func sqlGetProcessesWhere(dbType string) (string) {
	switch dbType {
	case "sqlite":
		return `SELECT * FROM processes WHERE host_id=?`
	case "postgres":
		return `SELECT * FROM processes WHERE host_id=$1`
	default:
		panic(fmt.Errorf("Incorrect database type!"))
	}
}

func (store *datastore) GetProcessesWhere(hostId int) (processes []Process, err error) {
	rows, err := store.Query(sqlGetProcessesWhere(store.DbType), hostId)
	if err != nil {
		return processes, err
	}

	defer rows.Close()
	for rows.Next() {
		process := Process{}
		err = rows.Scan(&process.id, &process.host_id, &process.process_id, &process.cmd)
		if err != nil {
			return processes, err
		}

		processes = append(processes, process)
	}

	return processes, rows.Err()
}

func sqlGetStatesWhere(dbType string) (string) {
	switch dbType {
	case "sqlite":
		return `SELECT * FROM states WHERE host_id=?`
	case "postgres":
		return `SELECT * FROM states WHERE host_id=$1`
	default:
		panic(fmt.Errorf("Incorrect database type!"))
	}
}

func (store *datastore) GetStatesWhere(hostId int) (states []State, err error) {
	rows, err := store.Query(sqlGetStatesWhere(store.DbType), hostId)
	if err != nil {
		return states, err
	}

	defer rows.Close()
	for rows.Next() {
		state := State{}
		err = rows.Scan(&state.id, &state.host_id, &state.process_id, &state.state)
		if err != nil {
			return states, err
		}

		states = append(states, state)
	}

	return states, rows.Err()
}