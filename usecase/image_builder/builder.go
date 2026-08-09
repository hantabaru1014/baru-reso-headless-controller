// Package image_builder は Resonite headless container image のローカルビルドを
// builder image への委譲で行う.
//
// container repo (baru-reso-headless-container) が発行する builder image
// (git / DepotDownloader / docker CLI + buildx を同梱) を Docker SDK で one-shot
// container として起動し、ホストの docker.sock を経由して inner build を走らせる.
// controller 自身は git clone / DepotDownloader / docker build を一切実行しない.
// 並列ビルドはプロセス内 mutex で排他する.
package image_builder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/api/types/container"
	"github.com/moby/moby/client"
)

const (
	// labelBuildID は builder が built image に焼く BUILD_ID label. 結果逆引きに使う.
	labelBuildID = "brhc.build-id"
	// labelImageTag は built image のタグ (name 抜き) label.
	labelImageTag = "brhc.image-tag"
	// labelResoniteVersion は built image の Resonite バージョン label.
	labelResoniteVersion = "brhc.resonite-version"
	// labelAppVersion は builder image / built image に焼かれる Headless/AppVersion label.
	labelAppVersion = "brhc.app-version"

	// builderSocketPath は builder container 内から見た docker.sock のマウント先 (契約で固定).
	builderSocketPath = "/var/run/docker.sock"

	// maxCapturedLogBytes は失敗時に保存する builder ログの上限. 超過分は先頭から
	// 捨てて末尾 (= 失敗箇所) を残す. DepotDownloader の進捗出力で膨らむため上限は必要.
	maxCapturedLogBytes = 2 << 20 // 2 MiB

	logTruncationNotice = "... (先頭を切り詰めました)\n"

	// minRedactableSecretLen 未満の credential はマスクしない. 短い値だと
	// 無関係な部分文字列まで壊してログが読めなくなるほうが害が大きい.
	minRedactableSecretLen = 4

	// logCaptureBufferFactor は capture バッファを limit の何倍確保するか.
	// 大きいほど詰め直しの頻度が下がる代わりにメモリを食う (ビルドは直列なので 1 本だけ).
	logCaptureBufferFactor = 2
)

// BuildError は builder container が非 0 で終了したことを表すエラー.
// 失敗時の builder ログ全文 (secrets はマスク済み) を詳細として持つ.
type BuildError struct {
	msg string
	log string
}

var _ domain.DetailedError = (*BuildError)(nil)

func (e *BuildError) Error() string { return e.msg }

func (e *BuildError) ErrorDetail() string { return e.log }

// Builder は builder image を起動して headless container image をビルドするサービス.
type Builder struct {
	cfg       *config.ResoniteBuildConfig
	dockerCfg *config.DockerConfig
	mu        sync.Mutex
}

func NewBuilder(cfg *config.ResoniteBuildConfig, dockerCfg *config.DockerConfig) *Builder {
	return &Builder{
		cfg:       cfg,
		dockerCfg: dockerCfg,
	}
}

// BuildParams は 1 回のビルド invocation の入力.
type BuildParams struct {
	ManifestID  string
	Branch      entity.ResoniteVersionBranch
	GameVersion *string // versions.json 由来. nil の場合は builder 側が Build.version から読む.
}

// BuildResult は成功時に返されるイメージ情報.
type BuildResult struct {
	ImageTag        string // <HEADLESS_IMAGE_NAME>:<computed> の <computed> 部分のみ
	ImageRef        string // 完全な <HEADLESS_IMAGE_NAME>:<computed>
	ResoniteVersion string
	AppVersion      string
}

// Build は 1 バージョンをビルドする. mutex で他ビルドと直列化.
func (b *Builder) Build(ctx context.Context, p BuildParams) (*BuildResult, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if p.ManifestID == "" {
		return nil, errors.New("manifest_id is required")
	}

	if err := b.validateCreds(); err != nil {
		return nil, err
	}

	cli, err := b.newDockerClient()
	if err != nil {
		return nil, errors.WrapPrefix(err, "create docker client", 0)
	}

	defer func() { _ = cli.Close() }()

	if err := b.ensureBuilderImage(ctx, cli); err != nil {
		return nil, err
	}

	buildID := uuid.NewString()

	slog.Info("image_builder: starting build",
		"manifest_id", p.ManifestID, "branch", p.Branch, "build_id", buildID)

	containerID, err := b.startBuilder(ctx, cli, p, buildID)
	if err != nil {
		return nil, err
	}

	defer func() {
		if _, rmErr := cli.ContainerRemove(context.WithoutCancel(ctx), containerID, client.ContainerRemoveOptions{
			Force: true,
		}); rmErr != nil {
			slog.Warn("image_builder: failed to remove builder container", "container", containerID, "err", rmErr)
		}
	}()

	captured := newLogCapture(maxCapturedLogBytes)

	if err := b.relayLogs(ctx, cli, containerID, captured); err != nil {
		slog.Warn("image_builder: failed to relay builder logs", "container", containerID, "err", err)
	}

	exitCode, err := b.waitBuilder(ctx, cli, containerID)
	if err != nil {
		return nil, err
	}

	if exitCode != 0 {
		// 一行サマリ (async_jobs.last_error / resonite_versions.build_error に載る) には
		// ログを含めない. ログ全文は BuildError の詳細として別途保存され、job 履歴の
		// エラー詳細からのみ参照される.
		return nil, &BuildError{
			msg: fmt.Sprintf("builder container %s exited with code %d", containerID, exitCode),
			log: sanitizeForStorage(b.redactSecrets(captured.String())),
		}
	}

	res, err := b.resultFromLabels(ctx, cli, buildID)
	if err != nil {
		return nil, err
	}

	slog.Info("image_builder: build succeeded", "image", res.ImageRef)

	return res, nil
}

// CurrentAppVersion は builder image の label brhc.app-version を読む.
// builder image を pull で最新化してから読むが, pull 失敗時はローカル既存で継続する.
func (b *Builder) CurrentAppVersion(ctx context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	cli, err := b.newDockerClient()
	if err != nil {
		return "", errors.WrapPrefix(err, "create docker client", 0)
	}

	defer func() { _ = cli.Close() }()

	if err := b.ensureBuilderImage(ctx, cli); err != nil {
		return "", err
	}

	inspect, err := cli.ImageInspect(ctx, b.cfg.BuilderImage)
	if err != nil {
		return "", errors.WrapPrefix(err, "inspect builder image", 0)
	}

	if inspect.Config == nil || inspect.Config.Labels[labelAppVersion] == "" {
		return "", errors.Errorf("builder image %s has no %s label", b.cfg.BuilderImage, labelAppVersion)
	}

	return inspect.Config.Labels[labelAppVersion], nil
}

// ensureBuilderImage は builder image の pull を試みる. pull に失敗しても
// ローカルに image があれば warn ログを出して継続, 無ければエラーを返す.
func (b *Builder) ensureBuilderImage(ctx context.Context, cli *client.Client) error {
	resp, pullErr := cli.ImagePull(ctx, b.cfg.BuilderImage, client.ImagePullOptions{})
	if pullErr == nil {
		// pull の進捗は読み切って完了を待つ (途中で close すると DL が中断される).
		if _, err := io.Copy(io.Discard, resp); err != nil {
			pullErr = err
		}

		_ = resp.Close()
	}

	if pullErr != nil {
		if _, inspectErr := cli.ImageInspect(ctx, b.cfg.BuilderImage); inspectErr != nil {
			return errors.Errorf("pull builder image %s failed and no local copy exists: %w", b.cfg.BuilderImage, pullErr)
		}

		slog.Warn("image_builder: failed to pull builder image; using local copy",
			"image", b.cfg.BuilderImage, "err", pullErr)
	}

	return nil
}

// startBuilder は builder container を作成・起動して container ID を返す.
func (b *Builder) startBuilder(ctx context.Context, cli *client.Client, p BuildParams, buildID string) (string, error) {
	// 毎回変わるパラメータは CLI 引数 (ENTRYPOINT への引数) で渡す. secrets のみ env.
	cmd := []string{
		"--manifest", p.ManifestID,
		"--branch", string(p.Branch),
		"--output-image", b.dockerCfg.HeadlessImageName,
		"--build-id", buildID,
		"--app-id", b.cfg.AppID,
	}

	if b.cfg.HeadlessDepotID != "" {
		cmd = append(cmd, "--depot-id", b.cfg.HeadlessDepotID)
	}

	if p.GameVersion != nil {
		cmd = append(cmd, "--game-version", *p.GameVersion)
	}

	env := []string{
		"STEAM_USERNAME=" + b.cfg.SteamUsername,
		"STEAM_PASSWORD=" + b.cfg.SteamPassword,
		"HEADLESS_PASSWORD=" + b.cfg.HeadlessPassword,
	}

	createResp, err := cli.ContainerCreate(ctx, client.ContainerCreateOptions{
		Config: &container.Config{
			Image: b.cfg.BuilderImage,
			Cmd:   cmd,
			Env:   env,
		},
		HostConfig: &container.HostConfig{
			// docker.sock の1本のみ. Resonite の DL 先 (/src/Resonite) は builder
			// container 内で ephemeral に扱う (契約: 永続 volume だと古い manifest の
			// 残骸が built image に混入しうるため).
			Binds: []string{
				b.cfg.DockerSocketPath + ":" + builderSocketPath,
			},
		},
	})
	if err != nil {
		return "", errors.WrapPrefix(err, "create builder container", 0)
	}

	if _, err := cli.ContainerStart(ctx, createResp.ID, client.ContainerStartOptions{}); err != nil {
		return "", errors.WrapPrefix(err, "start builder container", 0)
	}

	return createResp.ID, nil
}

// relayLogs は builder container のログ (stdout+stderr) を controller の
// os.Stdout / os.Stderr に転送しつつ, capture にも書き出す.
// Follow するため container 終了までブロックする.
func (b *Builder) relayLogs(ctx context.Context, cli *client.Client, containerID string, capture io.Writer) error {
	logs, err := cli.ContainerLogs(ctx, containerID, client.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Follow:     true,
	})
	if err != nil {
		return errors.WrapPrefix(err, "attach builder logs", 0)
	}

	defer func() { _ = logs.Close() }()

	// stdout / stderr は capture 上では 1 本に混ぜる. 失敗箇所の前後関係が
	// そのまま読めるほうがログとして有用なため.
	out := io.MultiWriter(os.Stdout, capture)
	errOut := io.MultiWriter(os.Stderr, capture)

	if _, err := stdcopy.StdCopy(out, errOut, logs); err != nil {
		return errors.WrapPrefix(err, "copy builder logs", 0)
	}

	return nil
}

// redactSecrets は builder に渡した credentials がログに現れていた場合に伏せる.
// entrypoint 側でも echo しない運用だが, ログを永続化する以上ここでも防御する.
func (b *Builder) redactSecrets(log string) string {
	secrets := []string{b.cfg.SteamPassword, b.cfg.HeadlessPassword, b.cfg.SteamUsername}

	for _, s := range secrets {
		if len(s) < minRedactableSecretLen {
			continue
		}

		log = strings.ReplaceAll(log, s, "***")
	}

	return log
}

// logCapture は「末尾 limit バイトだけ残す」Writer.
// builder ログは DepotDownloader の進捗等で数十 MB になりうる一方, 原因が出るのは
// 末尾なので, 溢れた分は先頭から捨てる.
//
// バッファは limit の 2 倍を最初に確保し, 溢れたときだけ末尾 limit バイトを先頭へ
// 詰め直す. 毎回 reslice で頭を捨てる素朴な実装だと backing array の容量が書き込む
// たびに減り, 数十 MB のログで growslice と limit バイトの memmove を繰り返す.
type logCapture struct {
	buf   []byte
	limit int
	// truncated は一度でも上限を超えたか (= 先頭を捨てたか).
	truncated bool
}

var _ io.Writer = (*logCapture)(nil)

func newLogCapture(limit int) *logCapture {
	return &logCapture{buf: make([]byte, 0, limit*logCaptureBufferFactor), limit: limit}
}

func (c *logCapture) Write(p []byte) (int, error) {
	c.buf = append(c.buf, p...)

	if len(c.buf) > c.limit {
		c.truncated = true
	}

	// 詰め直しは limit バイト書くごとに 1 回で済む.
	if len(c.buf) > c.limit*logCaptureBufferFactor {
		c.buf = c.buf[:copy(c.buf, c.buf[len(c.buf)-c.limit:])]
	}

	return len(p), nil
}

func (c *logCapture) String() string {
	if !c.truncated {
		return string(c.buf)
	}

	// 詰め直し直後を除き buf には limit を超える分が残っているので, ここで切る.
	cut := max(len(c.buf)-c.limit, 0)

	// 切断位置が行の途中 (= 直前が改行でない) なら, 欠けた先頭行ごと落とす.
	// 行どころか UTF-8 の途中で切れていることもあるので断片を持ち込まずに済み,
	// 万一 credential が境界を跨いで出力されていた場合に後段の redactSecrets が
	// その残骸を取りこぼすのも防げる.
	if cut > 0 && c.buf[cut-1] != '\n' {
		if nl := bytes.IndexByte(c.buf[cut:], '\n'); nl >= 0 {
			cut += nl + 1
		}
	}

	return logTruncationNotice + string(c.buf[cut:])
}

// sanitizeForStorage は Postgres の text 列に入らないバイトを落とす.
// builder のログは docker build 経由でバイナリを含みうるため, そのまま保存すると
// invalid byte sequence で書き込みごと失敗し, 肝心の失敗ログが残らなくなる.
func sanitizeForStorage(s string) string {
	s = strings.ToValidUTF8(s, "")

	return strings.ReplaceAll(s, "\x00", "")
}

// waitBuilder は container の終了を待ち exit code を返す.
func (b *Builder) waitBuilder(ctx context.Context, cli *client.Client, containerID string) (int64, error) {
	waitResult := cli.ContainerWait(ctx, containerID, client.ContainerWaitOptions{
		Condition: container.WaitConditionNotRunning,
	})

	select {
	case <-ctx.Done():
		return 0, errors.Wrap(ctx.Err(), 0)
	case err := <-waitResult.Error:
		return 0, errors.WrapPrefix(err, "wait builder container", 0)
	case res := <-waitResult.Result:
		return res.StatusCode, nil
	}
}

// resultFromLabels は BUILD_ID label で built image を逆引きし, label から BuildResult を組む.
func (b *Builder) resultFromLabels(ctx context.Context, cli *client.Client, buildID string) (*BuildResult, error) {
	images, err := cli.ImageList(ctx, client.ImageListOptions{
		Filters: make(client.Filters).Add("label", labelBuildID+"="+buildID),
	})
	if err != nil {
		return nil, errors.WrapPrefix(err, "list built image", 0)
	}

	if len(images.Items) == 0 {
		return nil, errors.Errorf("no image found with %s=%s (builder did not produce an image?)", labelBuildID, buildID)
	}

	if len(images.Items) > 1 {
		return nil, errors.Errorf("multiple images found with %s=%s", labelBuildID, buildID)
	}

	labels := images.Items[0].Labels

	tag := labels[labelImageTag]
	if tag == "" {
		return nil, errors.Errorf("built image is missing %s label", labelImageTag)
	}

	return &BuildResult{
		ImageTag:        tag,
		ImageRef:        fmt.Sprintf("%s:%s", b.dockerCfg.HeadlessImageName, tag),
		ResoniteVersion: labels[labelResoniteVersion],
		AppVersion:      labels[labelAppVersion],
	}, nil
}

func (b *Builder) validateCreds() error {
	if b.cfg.SteamUsername == "" || b.cfg.SteamPassword == "" || b.cfg.HeadlessPassword == "" {
		return errors.New("STEAM_USERNAME / STEAM_PASSWORD / HEADLESS_PASSWORD are required")
	}

	return nil
}

func (b *Builder) newDockerClient() (*client.Client, error) {
	cli, err := client.New(client.FromEnv)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}

	return cli, nil
}
