package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/skyfrost"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/spf13/cobra"
)

var portLabelKey = "dev.baru.brhdl.rpc-port"

func NewImportLegacyHostsCommand(q *db.Queries, skyfrostClient skyfrost.Client) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import-legacy-hosts",
		Short: "Import hosts from current docker containers",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Println("Importing legacy hosts...")
			cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
			if err != nil {
				cmd.PrintErrln("Error creating Docker client:", err)
				return
			}
			containers, err := cli.ContainerList(cmd.Context(), container.ListOptions{
				All:     true,
				Filters: filters.NewArgs(filters.Arg("label", portLabelKey)),
			})
			if err != nil {
				cmd.PrintErrln("Error listing Docker containers:", err)
				return
			}
			for _, c := range containers {
				if portValue, ok := c.Labels[portLabelKey]; ok && len(c.Names) > 0 {
					name := c.Names[0]
					if len(name) > 1 && name[0] == '/' {
						name = name[1:]
					}
					id := c.ID
					_, err := q.GetHost(cmd.Context(), id)
					if err == nil {
						// 既にインポート済みのホストはスキップ
						continue
					}

					inspectResult, err := cli.ContainerInspect(cmd.Context(), id)
					if err != nil {
						cmd.PrintErrln("Error inspecting container:", err)
						continue
					}
					var credential, password, startupConfig string
					for _, env := range inspectResult.Config.Env {
						if strings.HasPrefix(env, "HeadlessUserCredential=") {
							credential = strings.TrimPrefix(env, "HeadlessUserCredential=")
							continue
						}
						if strings.HasPrefix(env, "HeadlessUserPassword=") {
							password = strings.TrimPrefix(env, "HeadlessUserPassword=")
							continue
						}
						if strings.HasPrefix(env, "StartupConfig=") {
							startupConfig = strings.TrimPrefix(env, "StartupConfig=")
							continue
						}
					}
					if credential == "" || password == "" {
						cmd.PrintErrln("Host", name, "does not have HeadlessUserCredential or HeadlessUserPassword")
						continue
					}
					userSession, err := skyfrostClient.UserLogin(cmd.Context(), credential, password)
					if err != nil {
						cmd.PrintErrln("Error logging in headless account:", err)
						continue
					}
					status := int32(entity.HeadlessHostStatus_EXITED)
					if c.State == container.StateRunning {
						status = int32(entity.HeadlessHostStatus_RUNNING)
					}
					startedAt := pgtype.Timestamptz{}
					if parsedTime, err := time.Parse(time.RFC3339Nano, inspectResult.State.StartedAt); err == nil {
						startedAt = pgtype.Timestamptz{Time: parsedTime, Valid: true}
					}
					createParams := db.CreateHostParams{
						ID:                             id,
						Name:                           name,
						Status:                         status,
						AccountID:                      userSession.UserId,
						LastStartupConfigSchemaVersion: 1,
						ConnectorType:                  "docker",
						ConnectString:                  fmt.Sprintf("%s:%s", id, portValue),
						StartedAt:                      startedAt,
						AutoUpdatePolicy:               int32(entity.HostAutoUpdatePolicy_NEVER),
						Memo:                           pgtype.Text{String: "", Valid: true},
					}
					if startupConfig != "" {
						createParams.LastStartupConfig = []byte(startupConfig)
					} else {
						createParams.LastStartupConfig = []byte("{}")
					}
					_, err = q.CreateHost(cmd.Context(), createParams)
					if err != nil {
						cmd.PrintErrln("Error creating host:", err)
					}
					cmd.Printf("Imported host: %s (ID: %s)\n", name, id)
				}
			}
		},
	}

	return cmd
}
