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

	rootCmd.AddCommand(createUserCmd)
	rootCmd.AddCommand(deleteUserCmd)

	return rootCmd.Execute()
}
