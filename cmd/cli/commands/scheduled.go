package commands

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/actions"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/scheduled_op/triggers"
	"github.com/spf13/cobra"
)

func NewScheduledCommand(sou *usecase.ScheduledSessionOperationUsecase) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scheduled",
		Short: "Scheduled session operations (list / cancel / create)",
	}

	cmd.AddCommand(newScheduledListCmd(sou))
	cmd.AddCommand(newScheduledCancelCmd(sou))
	cmd.AddCommand(newScheduledCreateStopCmd(sou))

	return cmd
}

func newScheduledListCmd(sou *usecase.ScheduledSessionOperationUsecase) *cobra.Command {
	var (
		sessionID string
		hostID    string
		status    string
	)

	c := &cobra.Command{
		Use:   "list",
		Short: "List scheduled session operations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			filter := port.ScheduledSessionOperationListFilter{}
			if sessionID != "" {
				filter.SessionID = &sessionID
			}

			if hostID != "" {
				filter.HostID = &hostID
			}

			if status != "" {
				s, err := parseStatus(status)
				if err != nil {
					return err
				}

				filter.Status = &s
			}

			result, err := sou.List(ctx, filter)
			if err != nil {
				return err
			}

			if len(result.Items) == 0 {
				cmd.Println("No scheduled operations found")
				return nil
			}

			for _, e := range result.Items {
				printScheduledOp(cmd.OutOrStdout(), e)
			}

			return nil
		},
	}
	c.Flags().StringVar(&sessionID, "session", "", "filter by session_id")
	c.Flags().StringVar(&hostID, "host", "", "filter by host_id")
	c.Flags().StringVar(&status, "status", "", "filter by status (pending/running/succeeded/failed/canceled)")

	return c
}

func newScheduledCancelCmd(sou *usecase.ScheduledSessionOperationUsecase) *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <id>",
		Short: "Cancel a pending scheduled session operation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			if err := sou.Cancel(ctx, args[0]); err != nil {
				if errors.Is(err, usecase.ErrScheduledOperationNotCancelable) {
					return fmt.Errorf("not cancelable in its current status: %w", err)
				}

				return err
			}

			cmd.Printf("Canceled scheduled operation %s\n", args[0])

			return nil
		},
	}
}

// newScheduledCreateStopCmd は STOP_SESSION 予約を作る最小コマンド.
// START / UPDATE_PARAMETERS / UPDATE_EXTRA は引数が複雑なので CLI ではサポートせず WebUI 推奨.
// (CLI からも作成可能とのユーザー要望は、最低限 stop を作れる形で満たす).
func newScheduledCreateStopCmd(sou *usecase.ScheduledSessionOperationUsecase) *cobra.Command {
	var (
		sessionID string
		at        string
	)

	c := &cobra.Command{
		Use:   "create-stop",
		Short: "Schedule a STOP_SESSION at a specified time",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()

			if sessionID == "" {
				return errors.New("--session is required")
			}

			if at == "" {
				return errors.New("--at is required (RFC3339)")
			}

			t, err := time.Parse(time.RFC3339, at)
			if err != nil {
				return fmt.Errorf("invalid --at: %w", err)
			}

			act := actions.NewStopSessionAction(sessionID)
			trig := triggers.NewTimeTrigger(t)
			systemUserID := domain.SystemUserID

			created, err := sou.Create(ctx, usecase.CreateScheduledSessionOperationParams{
				Action:    act,
				Trigger:   trig,
				SessionID: &sessionID,
				CreatedBy: &systemUserID,
			})
			if err != nil {
				return err
			}

			cmd.Printf("Created scheduled stop operation %s at %s\n", created.ID, created.NextFireAt.Format(time.RFC3339))

			return nil
		},
	}
	c.Flags().StringVar(&sessionID, "session", "", "session_id to stop")
	c.Flags().StringVar(&at, "at", "", "scheduled_at (RFC3339, e.g. 2026-06-28T15:30:00+09:00)")

	return c
}

func parseStatus(s string) (entity.ScheduledOperationStatus, error) {
	switch strings.ToLower(s) {
	case "pending":
		return entity.ScheduledOperationStatus_PENDING, nil
	case "running":
		return entity.ScheduledOperationStatus_RUNNING, nil
	case "succeeded":
		return entity.ScheduledOperationStatus_SUCCEEDED, nil
	case "failed":
		return entity.ScheduledOperationStatus_FAILED, nil
	case "canceled":
		return entity.ScheduledOperationStatus_CANCELED, nil
	default:
		return 0, fmt.Errorf("unknown status: %s", s)
	}
}

func printScheduledOp(w io.Writer, e *entity.ScheduledSessionOperation) {
	row := struct {
		ID         string  `json:"id"`
		Operation  int32   `json:"operation"`
		Trigger    int32   `json:"trigger"`
		NextFireAt string  `json:"next_fire_at"`
		Status     int32   `json:"status"`
		HostID     *string `json:"host_id"`
		SessionID  *string `json:"session_id"`
		LastError  *string `json:"last_error"`
		ExecutedAt *string `json:"executed_at"`
	}{
		ID:         e.ID,
		Operation:  int32(e.OperationType),
		Trigger:    int32(e.TriggerType),
		NextFireAt: e.NextFireAt.Format(time.RFC3339),
		Status:     int32(e.Status),
		HostID:     e.HostID,
		SessionID:  e.SessionID,
		LastError:  e.LastError,
		ExecutedAt: timePtrOrNil(e.ExecutedAt),
	}

	b, err := json.Marshal(row)
	if err != nil {
		_, _ = fmt.Fprintf(w, "marshal error: %v\n", err)
		return
	}

	// 出力先 (stdout) 書き込みエラーは致命的ではないので無視.
	_, _ = fmt.Fprintln(w, string(b))
}

func timePtrOrNil(t *time.Time) *string {
	if t == nil {
		return nil
	}

	s := t.Format(time.RFC3339)

	return &s
}
