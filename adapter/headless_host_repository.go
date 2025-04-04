package adapter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var (
	imageName            = os.Getenv("HEADLESS_IMAGE_NAME")
	portLabelKey         = "dev.baru.brhdl.rpc-port"
	containerStopTimeout = 2 * 60
)

var _ port.HeadlessHostRepository = (*HeadlessHostRepository)(nil)

type HeadlessHostRepository struct {
	connections map[string]headlessv1.HeadlessControlServiceClient
}

func getFreePort() (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}

// Start implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) Start(ctx context.Context, params port.HeadlessHostStartParams) (string, error) {
	cli, err := h.newDockerClient()
	if err != nil {
		return "", fmt.Errorf("failed to create docker client: %w", err)
	}
	imageTag := "latest"
	if params.ContainerImageTag != nil {
		imageTag = *params.ContainerImageTag
	}
	port, err := getFreePort()
	if err != nil {
		return "", fmt.Errorf("failed to get free port: %w", err)
	}
	config := container.Config{
		Env: []string{
			fmt.Sprintf("RpcHostUrl=%s", fmt.Sprintf("http://localhost:%d", port)),
			fmt.Sprintf("HeadlessUserCredential=%s", params.HeadlessAccountCredential),
			fmt.Sprintf("HeadlessUserPassword=%s", params.HeadlessAccountPassword),
		},
		Image: fmt.Sprintf("%s:%s", imageName, imageTag),
		Labels: map[string]string{
			portLabelKey: fmt.Sprintf("%d", port),
		},
	}
	hostConfig := container.HostConfig{
		NetworkMode: "host",
	}
	createResp, err := cli.ContainerCreate(ctx, &config, &hostConfig, nil, nil, params.Name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}
	err = cli.ContainerStart(ctx, createResp.ID, container.StartOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return createResp.ID, nil
}

// Restart implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) Restart(ctx context.Context, host *entity.HeadlessHost) (string, error) {
	cli, err := h.newDockerClient()
	if err != nil {
		return "", fmt.Errorf("failed to create docker client: %w", err)
	}
	inspectResult, err := cli.ContainerInspect(ctx, host.ID)
	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}
	if host.Status == entity.HeadlessHostStatus_RUNNING {
		err = cli.ContainerStop(ctx, inspectResult.ID, container.StopOptions{
			Timeout: &containerStopTimeout,
		})
		if err != nil {
			return "", fmt.Errorf("failed to stop container: %w", err)
		}
	}
	err = cli.ContainerRemove(ctx, inspectResult.ID, container.RemoveOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to remove container: %w", err)
	}
	resp, err := cli.ContainerCreate(ctx, inspectResult.Config, inspectResult.HostConfig, nil, nil, host.Name)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}
	err = cli.ContainerStart(ctx, resp.ID, container.StartOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to start container: %w", err)
	}

	return resp.ID, nil
}

// PullContainerImage implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) PullContainerImage(ctx context.Context, tag string) error {
	cli, err := h.newDockerClient()
	if err != nil {
		return err
	}

	registryAuth := base64.StdEncoding.EncodeToString([]byte(os.Getenv("HEADLESS_REGISTRY_AUTH")))
	refStr := fmt.Sprintf("%s:%s", imageName, tag)
	_, err = cli.ImagePull(ctx, refStr, image.PullOptions{
		All:          false,
		RegistryAuth: registryAuth,
	})
	if err != nil {
		return err
	}

	return nil
}

// ListContainerTags implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) ListContainerTags(ctx context.Context, lastTag *string) ([]string, error) {
	type tagsResponse struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	imageNameParts := strings.Split(imageName, "/")
	if len(imageNameParts) != 3 {
		return nil, fmt.Errorf("invalid image name format: %s", imageName)
	}
	registryName := imageNameParts[0]
	userImagePair := strings.Join(imageNameParts[1:], "/")
	url := fmt.Sprintf("https://%s/v2/%s/tags/list", registryName, userImagePair)
	if lastTag != nil {
		url = fmt.Sprintf("%s?last=%s", url, *lastTag)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	if registryName == "ghcr.io" {
		authToken := os.Getenv("GHCR_AUTH_TOKEN")
		if authToken == "" {
			return nil, fmt.Errorf("GHCR_AUTH_TOKEN is not set")
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", base64.StdEncoding.EncodeToString([]byte(authToken))))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get tags: %s", resp.Status)
	}
	var tagsResp tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	tags := make([]string, 0, len(tagsResp.Tags))
	for _, tag := range tagsResp.Tags {
		if tag == "" {
			continue
		}
		tags = append(tags, tag)
	}

	return tags, nil
}

// Rename implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) Rename(ctx context.Context, id string, newName string) error {
	cli, err := h.newDockerClient()
	if err != nil {
		return err
	}
	return cli.ContainerRename(ctx, id, newName)
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
	container, err := h.getContainer(ctx, id)
	if err != nil {
		return nil, err
	}
	host, err := h.containerToEntity(ctx, *container)
	if err != nil {
		return nil, err
	}
	return host, nil
}

// ListAll implements repository.HeadlessHostRepository.
func (h *HeadlessHostRepository) ListAll(ctx context.Context) (entity.HeadlessHostList, error) {
	cli, err := h.newDockerClient()
	if err != nil {
		return nil, err
	}
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", portLabelKey)),
	})
	if err != nil {
		return nil, err
	}
	result := make(entity.HeadlessHostList, 0, len(containers))
	for _, c := range containers {
		entity, err := h.containerToEntity(ctx, c)
		if err != nil {
			continue
		}
		result = append(result, entity)
	}
	return result, nil
}

func NewHeadlessHostRepository() *HeadlessHostRepository {
	return &HeadlessHostRepository{
		connections: make(map[string]headlessv1.HeadlessControlServiceClient),
	}
}

func (h *HeadlessHostRepository) containerToEntity(ctx context.Context, container types.Container) (*entity.HeadlessHost, error) {
	if portValue, ok := container.Labels[portLabelKey]; ok && len(container.Names) > 0 {
		name := container.Names[0]
		if len(name) > 1 && name[0] == '/' {
			name = name[1:]
		}
		host := &entity.HeadlessHost{
			ID:      container.ID,
			Name:    name,
			Address: fmt.Sprintf("localhost:%s", portValue),
		}
		host.Status = containerStatusToEntityStatus(container.State, container.Status)
		if host.Status == entity.HeadlessHostStatus_RUNNING {
			if err := h.fetchHeadlessInfo(ctx, host); err != nil {
				return nil, err
			}
		}
		return host, nil
	}
	return nil, fmt.Errorf("required label %s not found", portLabelKey)
}

func containerStatusToEntityStatus(state, status string) entity.HeadlessHostStatus {
	switch state {
	case "running":
		return entity.HeadlessHostStatus_RUNNING
	case "exited":
		if strings.Contains(status, "Exited (0)") {
			return entity.HeadlessHostStatus_EXITED
		} else {
			return entity.HeadlessHostStatus_CRASHED
		}
	case "dead":
		return entity.HeadlessHostStatus_CRASHED
	default:
		return entity.HeadlessHostStatus_UNKNOWN
	}
}

func (h *HeadlessHostRepository) newDockerClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return cli, nil
}

func (h *HeadlessHostRepository) getContainer(ctx context.Context, id string) (*types.Container, error) {
	cli, err := h.newDockerClient()
	if err != nil {
		return nil, err
	}
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", portLabelKey), filters.Arg("id", id)),
	})
	if err != nil {
		return nil, err
	}
	if len(containers) > 1 {
		return nil, fmt.Errorf("found several containers")
	}
	return &containers[0], nil
}

func (h *HeadlessHostRepository) getOrNewHeadlessConnection(ctx context.Context, id string, address string) (headlessv1.HeadlessControlServiceClient, error) {
	container, err := h.getContainer(ctx, id)
	if err != nil {
		return nil, err
	}
	if conn, ok := h.connections[id]; ok {
		if container.State != "running" {
			delete(h.connections, id)
			return nil, fmt.Errorf("specific container is not running")
		}
		return conn, nil
	}
	if container.State != "running" {
		return nil, fmt.Errorf("specific container is not running")
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
