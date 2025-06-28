package app

import (
	"github.com/hantabaru1014/baru-reso-headless-controller/cmd/cli/commands"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/spf13/cobra"
)

type Cli struct {
	rootCmd *cobra.Command
}

func NewCli(queries *db.Queries, uu *usecase.UserUsecase, hu *usecase.HeadlessHostUsecase) *Cli {
	rootCmd := &cobra.Command{
		Use:   "brhcli",
		Short: "The CLI tool for baru-reso-headless-controller",
	}
	rootCmd.AddCommand(commands.NewHostCommand(hu))
	rootCmd.AddCommand(commands.NewUserCommand(uu))
	rootCmd.AddCommand(commands.NewMigrateCommand())
	rootCmd.AddCommand(commands.NewImportLegacyHostsCommand(queries))

	return &Cli{rootCmd: rootCmd}
}

func (c *Cli) Execute() error {
	return c.rootCmd.Execute()
}
