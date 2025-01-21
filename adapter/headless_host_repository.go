package adapter

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	imageName    = os.Getenv("HEADLESS_IMAGE_NAME")
	portLabelKey = "dev.baru.brhdl.rpc-port"
	dummyHost    = &entity.HeadlessHost{
		ID:      "1",
		Name:    os.Getenv("DUMMY_HOST_NAME"),
		Address: os.Getenv("DUMMY_HOST_ADDRESS"),
	}
)

var _ port.HeadlessHostRepository = (*HeadlessHostRepository)(nil)

type HeadlessHostRepository struct {
	connections map[string]headlessv1.HeadlessControlServiceClient
}

// GetLogs implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) GetLogs(ctx context.Context, id string, limit int32, until, since string) (port.LogLineList, error) {
	cli, err := h.newDockerClient()
	if err != nil {
		return nil, err
	}
	tail := fmt.Sprintf("%d", limit)
	if limit <= 0 {
		tail = "all"
	}
	r, err := cli.ContainerLogs(ctx, id, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Until:      until,
		Since:      since,
		Tail:       tail,
	})
	if err != nil {
		return nil, err
	}
	defer r.Close()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	written, err := stdcopy.StdCopy(stdout, stderr, r)
	if err != nil {
		return nil, err
	}
	// てきとーなサイズで初期化
	logs := make(port.LogLineList, 0, written/100)

	parseLogLine := func(line string, isError bool) (*port.LogLine, error) {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			return nil, fmt.Errorf("invalid log line format")
		}
		timestamp, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return nil, err
		}
		return &port.LogLine{
			Timestamp: timestamp.Unix(),
			IsError:   isError,
			Body:      parts[1],
		}, nil
	}

	readNextLogLine := func(buffer *bytes.Buffer, isError bool) (*port.LogLine, error) {
		line, err := buffer.ReadString('\n')
		if err == nil && len(line) > 0 {
			line = line[:len(line)-1]
		}
		if err != nil {
			return nil, err
		}
		return parseLogLine(line, isError)
	}

	var stdoutLog, stderrLog *port.LogLine
	var stdoutErr, stderrErr error

	for {
		if stdoutLog == nil && stdoutErr == nil {
			stdoutLog, stdoutErr = readNextLogLine(stdout, false)
		}
		if stderrLog == nil && stderrErr == nil {
			stderrLog, stderrErr = readNextLogLine(stderr, true)
		}

		if stdoutErr != nil && stderrErr != nil {
			break
		}

		if stdoutErr == nil && (stderrErr != nil || stdoutLog.Timestamp <= stderrLog.Timestamp) {
			logs = append(logs, stdoutLog)
			stdoutLog = nil
		} else {
			logs = append(logs, stderrLog)
			stderrLog = nil
		}
	}

	return logs, nil
}

// GetRpcClient implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) GetRpcClient(ctx context.Context, id string) (headlessv1.HeadlessControlServiceClient, error) {
	if conn, ok := h.connections[id]; ok {
		return conn, nil
	}

	host, err := h.Find(ctx, id)
	if err != nil {
		return nil, err
	}
	return h.getOrNewHeadlessConnection(ctx, host.ID, host.Address)
}

// Find implements repository.HeadlessHostRepository.
func (h *HeadlessHostRepository) Find(ctx context.Context, id string) (*entity.HeadlessHost, error) {
	if id == dummyHost.ID {
		return dummyHost, nil
	}
	cli, err := h.newDockerClient()
	if err != nil {
		return nil, err
	}
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("ancestor", imageName), filters.Arg("id", id)),
	})
	if err != nil {
		return nil, err
	}
	if len(containers) == 1 {
		host := &entity.HeadlessHost{
			ID:      containers[0].ID,
			Name:    containers[0].Names[0],
			Address: fmt.Sprintf("localhost:%s", containers[0].Labels[portLabelKey]),
		}
		if err := h.fetchHeadlessInfo(ctx, host); err != nil {
			return nil, err
		}

		return host, nil
	}
	return dummyHost, nil
}

// ListAll implements repository.HeadlessHostRepository.
func (h *HeadlessHostRepository) ListAll(ctx context.Context) (entity.HeadlessHostList, error) {
	cli, err := h.newDockerClient()
	if err != nil {
		return nil, err
	}
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		Filters: filters.NewArgs(filters.Arg("ancestor", imageName)),
	})
	if err != nil {
		return nil, err
	}
	result := make(entity.HeadlessHostList, 0, len(containers)+1)
	// result = append(result, dummyHost)
	for _, c := range containers {
		if portValue, ok := c.Labels[portLabelKey]; ok && len(c.Names) > 0 {
			host := &entity.HeadlessHost{
				ID:      c.ID,
				Name:    c.Names[0],
				Address: fmt.Sprintf("localhost:%s", portValue),
			}
			if err := h.fetchHeadlessInfo(ctx, host); err != nil {
				continue
			}
			result = append(result, host)
		}
	}
	return result, nil
}

func NewHeadlessHostRepository() *HeadlessHostRepository {
	return &HeadlessHostRepository{
		connections: make(map[string]headlessv1.HeadlessControlServiceClient),
	}
}

func (h *HeadlessHostRepository) newDockerClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return cli, nil
}

func (h *HeadlessHostRepository) getOrNewHeadlessConnection(_ context.Context, id string, address string) (headlessv1.HeadlessControlServiceClient, error) {
	if conn, ok := h.connections[id]; ok {
		return conn, nil
	}

	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	client := headlessv1.NewHeadlessControlServiceClient(conn)
	h.connections[id] = client

	return client, nil
}

func (h *HeadlessHostRepository) fetchHeadlessInfo(ctx context.Context, host *entity.HeadlessHost) error {
	conn, err := h.getOrNewHeadlessConnection(ctx, host.ID, host.Address)
	if err != nil {
		return err
	}

	info, err := conn.GetAbout(ctx, &headlessv1.GetAboutRequest{})
	if err != nil {
		return err
	}
	host.ResoniteVersion = info.ResoniteVersion

	accountInfo, err := conn.GetAccountInfo(ctx, &headlessv1.GetAccountInfoRequest{})
	if err != nil {
		return err
	}
	host.AccountId = accountInfo.UserId
	host.AccountName = accountInfo.DisplayName
	host.StorageQuotaBytes = accountInfo.StorageQuotaBytes
	host.StorageUsedBytes = accountInfo.StorageUsedBytes

	status, err := conn.GetStatus(ctx, &headlessv1.GetStatusRequest{})
	if err != nil {
		return err
	}
	host.Fps = status.Fps

	return nil
}
