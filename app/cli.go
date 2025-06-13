package app

import (
	"os"
	"sync"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/spf13/cobra"
)

type Cli struct {
	uu *usecase.UserUsecase
	hu *usecase.HeadlessHostUsecase
}

func NewCli(uu *usecase.UserUsecase, hu *usecase.HeadlessHostUsecase) *Cli {
	return &Cli{uu: uu, hu: hu}
}

var (
	rootCmd = &cobra.Command{
		Use:   "brhcli",
		Short: "The CLI tool for baru-reso-headless-controller",
	}
)

func (c *Cli) Execute() error {
	// User commands
	userCmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
	}

	createUserCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a user",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			err := c.uu.CreateUser(cmd.Context(), args[0], args[1], args[2])
			if err != nil {
				cmd.PrintErrln(err)
				return
			}
			_, err = c.uu.GetUserWithPassword(cmd.Context(), args[0], args[1])
			if err != nil {
				cmd.PrintErrln("Failed validate created user:", err)
				return
			}
			cmd.Println("User created successfully")
		},
	}

	deleteUserCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err := c.uu.DeleteUser(cmd.Context(), args[0])
			if err != nil {
				cmd.PrintErrln(err)
				return
			}
			cmd.Println("User deleted successfully")
		},
	}

	userCmd.AddCommand(createUserCmd)
	userCmd.AddCommand(deleteUserCmd)

	// Host commands
	hostCmd := &cobra.Command{
		Use:   "host",
		Short: "Headless host management commands",
	}

	listHostsCmd := &cobra.Command{
		Use:   "list",
		Short: "List all headless hosts",
		Run: func(cmd *cobra.Command, args []string) {
			hosts, err := c.hu.HeadlessHostList(cmd.Context())
			if err != nil {
				cmd.PrintErrln("Failed to list hosts:", err)
				return
			}
			if len(hosts) == 0 {
				cmd.Println("No hosts found")
				return
			}
			for _, host := range hosts {
				status := "UNKNOWN"
				switch host.Status {
				case entity.HeadlessHostStatus_STARTING:
					status = "STARTING"
				case entity.HeadlessHostStatus_RUNNING:
					status = "RUNNING"
				case entity.HeadlessHostStatus_STOPPING:
					status = "STOPPING"
				case entity.HeadlessHostStatus_EXITED:
					status = "EXITED"
				case entity.HeadlessHostStatus_CRASHED:
					status = "CRASHED"
				}
				cmd.Printf("ID: %s, Name: %s, Status: %s\n", host.ID, host.Name, status)
			}
		},
	}

	restartAllHostsCmd := &cobra.Command{
		Use:   "restart-all",
		Short: "Restart all headless hosts",
		Run: func(cmd *cobra.Command, args []string) {
			timeout, err := cmd.Flags().GetInt("timeout")
			if err != nil {
				cmd.PrintErrln("Invalid timeout value:", err)
				return
			}

			hosts, err := c.hu.HeadlessHostList(cmd.Context())
			if err != nil {
				cmd.PrintErrln("Failed to list hosts:", err)
				return
			}

			var wg sync.WaitGroup
			for _, host := range hosts {
				wg.Add(1)
				go func(h *entity.HeadlessHost) {
					defer wg.Done()
					err := c.hu.HeadlessHostRestart(cmd.Context(), h.ID, nil, true, timeout)
					if err != nil {
						cmd.PrintErrf("Failed to restart host %s: %v\n", h.Name, err)
						return
					}
					cmd.Printf("Host %s restarted successfully\n", h.Name)
				}(host)
			}
			wg.Wait()
			cmd.Println("All hosts restarted successfully")
		},
	}
	restartAllHostsCmd.Flags().Int("timeout", 600, "Timeout in seconds for restart operation")

	shutdownAllHostsCmd := &cobra.Command{
		Use:   "shutdown-all",
		Short: "Shutdown all headless hosts",
		Run: func(cmd *cobra.Command, args []string) {
			hosts, err := c.hu.HeadlessHostList(cmd.Context())
			if err != nil {
				cmd.PrintErrln("Failed to list hosts:", err)
				return
			}
			var wg sync.WaitGroup
			for _, host := range hosts {
				wg.Add(1)
				go func(h *entity.HeadlessHost) {
					defer wg.Done()
					err := c.hu.HeadlessHostShutdown(cmd.Context(), h.ID)
					if err != nil {
						cmd.PrintErrf("Failed to shutdown host %s: %v\n", h.Name, err)
						return
					}
					cmd.Printf("Host %s shutdown successfully\n", h.Name)
				}(host)
			}
			wg.Wait()
			cmd.Println("All hosts shutdown successfully")
		},
	}

	hostCmd.AddCommand(listHostsCmd)
	hostCmd.AddCommand(restartAllHostsCmd)
	hostCmd.AddCommand(shutdownAllHostsCmd)

	// Database migration command
	migrateCmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate database",
		Run: func(cmd *cobra.Command, args []string) {
			d, err := iofs.New(db.MigrationFiles, "migrations")
			if err != nil {
				cmd.PrintErrln(err)
				return
			}
			m, err := migrate.NewWithSourceInstance("iofs", d, os.Getenv("DB_URL"))
			if err != nil {
				cmd.PrintErrln(err)
				return
			}
			err = m.Up()
			if err != nil {
				if err != migrate.ErrNoChange {
					cmd.PrintErrln(err)
					return
				} else {
					cmd.Println("No changed")
					return
				}
			}
			cmd.Println("Migrated successfully")
		},
	}

	rootCmd.AddCommand(userCmd)
	rootCmd.AddCommand(hostCmd)
	rootCmd.AddCommand(migrateCmd)

	return rootCmd.Execute()
}
