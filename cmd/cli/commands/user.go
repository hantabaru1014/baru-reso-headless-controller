package commands

import (
	"fmt"
	"os"

	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/spf13/cobra"
)

func NewUserCommand(uu *usecase.UserUsecase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "user",
		Short: "User management commands",
	}

	inviteCmd := &cobra.Command{
		Use:   "invite <resoniteId>",
		Short: "Generate a registration link for a new user",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			resoniteId := args[0]
			token, err := uu.CreateRegistrationToken(cmd.Context(), resoniteId)
			if err != nil {
				cmd.PrintErrln("Failed to create registration token:", err)
				return
			}

			frontUrl := os.Getenv("FRONT_URL")
			if frontUrl == "" {
				frontUrl = "http://localhost:5173"
			}

			registrationUrl := fmt.Sprintf("%s/register/%s", frontUrl, token)
			cmd.Println("Registration link generated successfully!")
			cmd.Println("Resonite ID:", resoniteId)
			cmd.Println("Valid for: 24 hours")
			cmd.Println("")
			cmd.Println("Registration URL:")
			cmd.Println(registrationUrl)
		},
	}

	createUserCmd := &cobra.Command{
		Use:   "create <id> <password> <resoniteId>",
		Short: "Create a user directly (deprecated, use 'invite' instead)",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {
			err := uu.CreateUser(cmd.Context(), args[0], args[1], args[2])
			if err != nil {
				cmd.PrintErrln(err)
				return
			}
			_, err = uu.GetUserWithPassword(cmd.Context(), args[0], args[1])
			if err != nil {
				cmd.PrintErrln("Failed validate created user:", err)
				return
			}
			cmd.Println("User created successfully")
		},
	}

	deleteUserCmd := &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a user",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			err := uu.DeleteUser(cmd.Context(), args[0])
			if err != nil {
				cmd.PrintErrln(err)
				return
			}
			cmd.Println("User deleted successfully")
		},
	}

	cmd.AddCommand(inviteCmd, createUserCmd, deleteUserCmd)
	return cmd
}
