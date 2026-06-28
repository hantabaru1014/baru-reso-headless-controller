package commands

import (
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/spf13/cobra"
)

// NewSystemAdminCommand は `brhcli system-admin add|remove <userID>` を提供する.
// system グループに seed-system-admin で追加 / 削除する CLI.
func NewSystemAdminCommand(guc *usecase.GroupUsecase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "system-admin",
		Short: "Manage system group admin members",
	}

	addCmd := &cobra.Command{
		Use:   "add <userID>",
		Short: "Grant system-admin role to a user",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			addedBy := domain.SystemUserID

			if _, err := guc.AddGroupMember(ctx,
				entity.SystemGroupID,
				args[0],
				entity.SeedRoleID_SystemAdmin,
				&addedBy,
			); err != nil {
				cmd.PrintErrln("Failed to add system-admin:", err)
				return
			}

			cmd.Println("system-admin granted to", args[0])
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove <userID>",
		Short: "Revoke system-admin role from a user",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()

			if err := guc.RemoveGroupMember(ctx, entity.SystemGroupID, args[0]); err != nil {
				cmd.PrintErrln("Failed to remove system-admin:", err)
				return
			}

			cmd.Println("system-admin revoked from", args[0])
		},
	}

	cmd.AddCommand(addCmd, removeCmd)

	return cmd
}
