package app

import (
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/spf13/cobra"
)

type Cli struct {
	uu *usecase.UserUsecase
}

func NewCli(uu *usecase.UserUsecase) *Cli {
	return &Cli{uu: uu}
}

var (
	rootCmd = &cobra.Command{
		Use:   "brhcli",
		Short: "The CLI tool for baru-reso-headless-controller",
	}
)

func (c *Cli) Execute() error {
	createUserCmd := &cobra.Command{
		Use:   "create-user",
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
		Use:   "delete-user",
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

	rootCmd.AddCommand(createUserCmd)
	rootCmd.AddCommand(deleteUserCmd)
	rootCmd.AddCommand(migrateCmd)

	return rootCmd.Execute()
}
