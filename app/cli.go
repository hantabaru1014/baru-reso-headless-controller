package app

import (
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
			cmd.Println("User created successfully")
		},
	}

	rootCmd.AddCommand(createUserCmd)

	return rootCmd.Execute()
}
