package commands

import (
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/spf13/cobra"
)

func NewMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
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

	return cmd
}
