package hostconnector

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
)

var (
	imageName = os.Getenv("HEADLESS_IMAGE_NAME")
)

var _ HostConnector = (*DockerHostConnector)(nil)

type DockerHostConnector struct {
}

// GetStatus implements HostConnector.
func (d *DockerHostConnector) GetStatus(ctx context.Context, connect_string HostConnectString) entity.HeadlessHostStatus {
	container, err := d.getContainer(ctx, string(connect_string))
	if err != nil {
		return entity.HeadlessHostStatus_UNKNOWN
	}
	return containerStatusToEntityStatus(container.State, container.Status)
}

// GetLogs implements HostConnector.
func (d *DockerHostConnector) GetLogs(ctx context.Context, connect_string HostConnectString, limit int32, until string, since string) (port.LogLineList, error) {
	cli, err := d.newDockerClient()
	if err != nil {
		return nil, errors.New(err)
	}
	tail := fmt.Sprintf("%d", limit)
	if limit <= 0 {
		tail = "all"
	}
	r, err := cli.ContainerLogs(ctx, string(connect_string), container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: true,
		Until:      until,
		Since:      since,
		Tail:       tail,
	})
	if err != nil {
		return nil, errors.New(err)
	}
	defer r.Close()

	stdout := new(bytes.Buffer)
	stderr := new(bytes.Buffer)
	written, err := stdcopy.StdCopy(stdout, stderr, r)
	if err != nil {
		return nil, errors.New(err)
	}
	// てきとーなサイズで初期化
	logs := make(port.LogLineList, 0, written/100)

	parseLogLine := func(line string, isError bool) (*port.LogLine, error) {
		parts := strings.SplitN(line, " ", 2)
		if len(parts) < 2 {
			return nil, errors.Errorf("invalid log line format")
		}
		timestamp, err := time.Parse(time.RFC3339Nano, parts[0])
		if err != nil {
			return nil, errors.New(err)
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
			return nil, errors.New(err)
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

// GetRpcClient implements HostConnector.
func (d *DockerHostConnector) GetRpcClient(ctx context.Context, connect_string HostConnectString) (headlessv1.HeadlessControlServiceClient, error) {
	splitted := strings.Split(string(connect_string), ":")
	if len(splitted) != 2 {
		return nil, errors.Errorf("invalid connect string format: %s", connect_string)
	}
	container, err := d.getContainer(ctx, splitted[0])
	if err != nil {
		return nil, errors.Errorf("failed to get container: %w", err)
	}
	if container.State != "running" {
		return nil, errors.Errorf("specific container is not running")
	}
	if container.Labels == nil {
		return nil, errors.Errorf("container labels are nil")
	}
	port, err := strconv.Atoi(splitted[1])
	if err != nil {
		return nil, errors.Errorf("invalid port format: %s", splitted[1])
	}
	address := fmt.Sprintf("localhost:%d", port)
	conn, err := grpc.NewClient(address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, errors.New(err)
	}
	return headlessv1.NewHeadlessControlServiceClient(conn), nil
}

func (d *DockerHostConnector) ListContainerTags(ctx context.Context, lastTag *string) (port.ContainerImageList, error) {
	type tagsResponse struct {
		Name string   `json:"name"`
		Tags []string `json:"tags"`
	}

	imageNameParts := strings.Split(imageName, "/")
	if len(imageNameParts) != 3 {
		return nil, errors.Errorf("invalid image name format: %s", imageName)
	}
	registryName := imageNameParts[0]
	userImagePair := strings.Join(imageNameParts[1:], "/")
	url := fmt.Sprintf("https://%s/v2/%s/tags/list", registryName, userImagePair)
	if lastTag != nil {
		url = fmt.Sprintf("%s?last=%s", url, *lastTag)
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, errors.Errorf("failed to create request: %w", err)
	}

	if registryName == "ghcr.io" {
		authToken := os.Getenv("GHCR_AUTH_TOKEN")
		if authToken == "" {
			return nil, errors.Errorf("GHCR_AUTH_TOKEN is not set")
		}
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", base64.StdEncoding.EncodeToString([]byte(authToken))))
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, errors.Errorf("failed to get tags: %s", resp.Status)
	}
	var tagsResp tagsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tagsResp); err != nil {
		return nil, errors.Errorf("failed to decode response: %w", err)
	}

	tags := make(port.ContainerImageList, 0, len(tagsResp.Tags))
	for _, tag := range tagsResp.Tags {
		info := parseTag(tag)
		if info.IsVersioned {
			tags = append(tags, &port.ContainerImage{
				Tag:             info.Tag,
				ResoniteVersion: info.ResoniteVersion,
				IsPreRelease:    info.IsPreRelease,
				AppVersion:      info.AppVersion,
			})
		}
	}

	return tags, nil
}

func (d *DockerHostConnector) PullContainerImage(ctx context.Context, tag string) (string, error) {
	cli, err := d.newDockerClient()
	if err != nil {
		return "", errors.New(err)
	}

	registryAuth := base64.StdEncoding.EncodeToString([]byte(os.Getenv("HEADLESS_REGISTRY_AUTH")))
	refStr := fmt.Sprintf("%s:%s", imageName, tag)
	reader, err := cli.ImagePull(ctx, refStr, image.PullOptions{
		All:          false,
		RegistryAuth: registryAuth,
	})
	if err != nil {
		return "", errors.New(err)
	}
	buf := new(bytes.Buffer)
	io.Copy(buf, reader)

	return buf.String(), nil
}

// Start implements HostConnector.
func (d *DockerHostConnector) Start(ctx context.Context, params port.HeadlessHostStartParams) (HostConnectString, error) {
	cli, err := d.newDockerClient()
	if err != nil {
		return "", errors.Errorf("failed to create docker client: %w", err)
	}
	imageTag := params.ContainerImageTag
	if !d.isAvailableTag(ctx, imageTag) {
		_, err := d.PullContainerImage(ctx, imageTag)
		if err != nil {
			return "", errors.Errorf("failed to pull container image: %w", err)
		}
	}
	port, err := getFreePort()
	if err != nil {
		return "", errors.Errorf("failed to get free port: %w", err)
	}
	var startupConfig *string
	if params.StartupConfig != nil {
		configJson, err := protojson.Marshal(params.StartupConfig)
		if err != nil {
			return "", errors.Errorf("failed to marshal startup config: %w", err)
		}
		str := string(configJson)
		startupConfig = &str
	}
	envs := []string{
		fmt.Sprintf("RpcHostUrl=%s", fmt.Sprintf("http://localhost:%d", port)),
		fmt.Sprintf("HeadlessUserCredential=%s", params.HeadlessAccount.Credential),
		fmt.Sprintf("HeadlessUserPassword=%s", params.HeadlessAccount.Password),
	}
	if startupConfig != nil {
		envs = append(envs, fmt.Sprintf("StartupConfig=%s", *startupConfig))
	}
	config := container.Config{
		Env:   envs,
		Image: fmt.Sprintf("%s:%s", imageName, imageTag),
	}
	hostConfig := container.HostConfig{
		NetworkMode: "host",
	}
	createResp, err := cli.ContainerCreate(ctx, &config, &hostConfig, nil, nil, "")
	if err != nil {
		return "", errors.Errorf("failed to create container: %w", err)
	}
	err = cli.ContainerStart(ctx, createResp.ID, container.StartOptions{})
	if err != nil {
		return "", errors.Errorf("failed to start container: %w", err)
	}

	return HostConnectString(createResp.ID + fmt.Sprintf(":%d", port)), nil
}

// Stop implements HostConnector.
func (d *DockerHostConnector) Stop(ctx context.Context, connect_string HostConnectString, timeoutSeconds int) error {
	cli, err := d.newDockerClient()
	if err != nil {
		return errors.Errorf("failed to create docker client: %w", err)
	}
	err = cli.ContainerStop(ctx, string(connect_string), container.StopOptions{
		Timeout: &timeoutSeconds,
	})
	if err != nil {
		return errors.Errorf("failed to stop container: %w", err)
	}

	return nil
}

func NewDockerHostConnector() *DockerHostConnector {
	return &DockerHostConnector{}
}

type TagInfo struct {
	Tag             string
	IsVersioned     bool
	IsPreRelease    bool
	ResoniteVersion string
	AppVersion      string
}

// TODO: imageに情報を埋め込んだらタグ名からパースするのをやめる
func parseTag(tag string) TagInfo {
	trimmed := strings.TrimPrefix(tag, "prerelease-")
	splitted := strings.Split(trimmed, "-")
	appVersion := "v0.0.0"
	if len(splitted) == 2 {
		appVersion = splitted[1]
	}
	if len(splitted) > 0 && lib.ValidateResoniteVersionString(splitted[0]) {
		return TagInfo{
			Tag:             tag,
			IsVersioned:     true,
			IsPreRelease:    strings.HasPrefix(tag, "prerelease-"),
			ResoniteVersion: splitted[0],
			AppVersion:      appVersion,
		}
	} else {
		return TagInfo{
			Tag:             tag,
			IsVersioned:     false,
			IsPreRelease:    false,
			ResoniteVersion: "",
		}
	}
}

// 指定したタグがローカルに存在するかどうかを確認する
func (d *DockerHostConnector) isAvailableTag(ctx context.Context, tag string) bool {
	cli, err := d.newDockerClient()
	if err != nil {
		return false
	}
	images, err := cli.ImageList(ctx, image.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("reference", imageName)),
	})
	if err != nil {
		return false
	}
	for _, img := range slices.Backward(images) {
		for _, repoTag := range img.RepoTags {
			if strings.HasPrefix(repoTag, imageName) {
				splitted := strings.Split(repoTag, ":")
				if len(splitted) != 2 {
					continue
				}
				if splitted[1] == tag {
					return true
				}
			}
		}
	}
	return false
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

func (d *DockerHostConnector) newDockerClient() (*client.Client, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return cli, nil
}

func (d *DockerHostConnector) getContainer(ctx context.Context, container_id string) (*container.Summary, error) {
	cli, err := d.newDockerClient()
	if err != nil {
		return nil, errors.New(err)
	}
	containers, err := cli.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("id", container_id)),
	})
	if err != nil {
		return nil, errors.New(err)
	}
	if len(containers) == 0 {
		return nil, errors.Errorf("container not found")
	}
	if len(containers) > 1 {
		return nil, errors.Errorf("found several containers")
	}
	return &containers[0], nil
}
