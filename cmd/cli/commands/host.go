package commands

import (
	"sync"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/spf13/cobra"
)

func NewHostCommand(hu *usecase.HeadlessHostUsecase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "host",
		Short: "Headless host management commands",
	}

	listHostsCmd := &cobra.Command{
		Use:   "list",
		Short: "List all headless hosts",
		Run: func(cmd *cobra.Command, args []string) {
			hosts, err := hu.HeadlessHostList(cmd.Context())
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

			hosts, err := hu.HeadlessHostList(cmd.Context())
			if err != nil {
				cmd.PrintErrln("Failed to list hosts:", err)
				return
			}

			var wg sync.WaitGroup
			for _, host := range hosts {
				wg.Add(1)
				go func(h *entity.HeadlessHost) {
					defer wg.Done()
					err := hu.HeadlessHostRestart(cmd.Context(), h.ID, nil, true, timeout)
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
			hosts, err := hu.HeadlessHostList(cmd.Context())
			if err != nil {
				cmd.PrintErrln("Failed to list hosts:", err)
				return
			}
			var wg sync.WaitGroup
			for _, host := range hosts {
				wg.Add(1)
				go func(h *entity.HeadlessHost) {
					defer wg.Done()
					err := hu.HeadlessHostShutdown(cmd.Context(), h.ID)
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

	cmd.AddCommand(listHostsCmd, restartAllHostsCmd, shutdownAllHostsCmd)

	return cmd
}
