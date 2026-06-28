package app

import (
	"github.com/hantabaru1014/baru-reso-headless-controller/cmd/cli/commands"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/auth"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/spf13/cobra"
)

type Cli struct {
	rootCmd *cobra.Command
}

func NewCli(
	queries *db.Queries,
	uu *usecase.UserUsecase,
	hu *usecase.HeadlessHostUsecase,
	sou *usecase.ScheduledSessionOperationUsecase,
	guc *usecase.GroupUsecase,
	skyfrostClient skyfrost.Client,
) *Cli {
	rootCmd := &cobra.Command{
		Use:   "brhcli",
		Short: "The CLI tool for baru-reso-headless-controller",
		// 全 CLI コマンドの実行主体を system user に固定する.
		// 子コマンドの Run/RunE は cmd.Context() からそのまま claims を引ける.
		PersistentPreRun: func(cmd *cobra.Command, _ []string) {
			cmd.SetContext(auth.WithActAsUser(cmd.Context(), domain.SystemUserID))
		},
	}
	rootCmd.AddCommand(commands.NewHostCommand(hu))
	rootCmd.AddCommand(commands.NewUserCommand(uu, skyfrostClient))
	rootCmd.AddCommand(commands.NewMigrateCommand())
	rootCmd.AddCommand(commands.NewImportLegacyHostsCommand(queries, skyfrostClient))
	rootCmd.AddCommand(commands.NewScheduledCommand(sou))
	rootCmd.AddCommand(commands.NewSystemAdminCommand(guc))

	return &Cli{rootCmd: rootCmd}
}

func (c *Cli) Execute() error {
	return c.rootCmd.Execute()
}
