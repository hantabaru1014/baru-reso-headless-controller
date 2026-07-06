// Package image_builder は Resonite headless container のローカルビルドを担う.
//
// container repo (baru-reso-headless-container) を clone/fetch し、DepotDownloader で
// 指定 manifest の Resonite を取得、native-libs を整形して docker build を実行する.
// 並列ビルドはプロセス内 mutex で排他する.
package image_builder

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/go-errors/errors"
	"github.com/hantabaru1014/baru-reso-headless-controller/config"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
)

// Builder は container repo と DepotDownloader を組み合わせて image をビルドするサービス.
type Builder struct {
	cfg       *config.ResoniteBuildConfig
	dockerCfg *config.DockerConfig
	repoPath  string // 絶対パス (cfg.ContainerRepoPath の Abs 版; 初回 EnsureRepo で lazily resolve).
	mu        sync.Mutex
}

func NewBuilder(cfg *config.ResoniteBuildConfig, dockerCfg *config.DockerConfig) *Builder {
	return &Builder{
		cfg:       cfg,
		dockerCfg: dockerCfg,
	}
}

// resolveRepoPath は cfg.ContainerRepoPath を絶対パスに正規化して repoPath に保持する.
// cmd.Dir と組み合わせる際にパス崩れが起きないよう常に絶対にしておく.
func (b *Builder) resolveRepoPath() (string, error) {
	if b.repoPath != "" {
		return b.repoPath, nil
	}

	p := b.cfg.ContainerRepoPath
	if p == "" {
		return "", errors.New("container repo path is not configured")
	}

	abs, err := filepath.Abs(p)
	if err != nil {
		return "", errors.WrapPrefix(err, "resolve container repo path", 0)
	}

	b.repoPath = abs

	return abs, nil
}

// EnsureRepo は container repo を clone (未存在時) / fetch + reset で最新化する.
// mutex 内で呼ぶ想定 (Build と CurrentAppVersion からのみ呼ばれる).
func (b *Builder) EnsureRepo(ctx context.Context) error {
	path, err := b.resolveRepoPath()
	if err != nil {
		return err
	}

	gitDir := filepath.Join(path, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		if mkErr := os.MkdirAll(filepath.Dir(path), 0o755); mkErr != nil {
			return errors.WrapPrefix(mkErr, "mkdir parent", 0)
		}

		if err := runCmd(ctx, "", "git", "clone", "--depth", "1", "--branch", b.cfg.ContainerRepoRef, b.cfg.ContainerRepoURL, path); err != nil {
			return errors.WrapPrefix(err, "git clone", 0)
		}

		return nil
	}

	if err := runCmd(ctx, path, "git", "fetch", "--depth", "1", "origin", b.cfg.ContainerRepoRef); err != nil {
		return errors.WrapPrefix(err, "git fetch", 0)
	}

	if err := runCmd(ctx, path, "git", "reset", "--hard", "FETCH_HEAD"); err != nil {
		return errors.WrapPrefix(err, "git reset", 0)
	}

	return nil
}

// CurrentAppVersion は container repo に checkout 済みの Headless/AppVersion を読む.
// EnsureRepo を先に呼んでおくこと.
func (b *Builder) CurrentAppVersion(ctx context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := b.EnsureRepo(ctx); err != nil {
		return "", err
	}

	return b.readAppVersion()
}

func (b *Builder) readAppVersion() (string, error) {
	p := filepath.Join(b.repoPath, "Headless", "AppVersion")

	data, err := os.ReadFile(p)
	if err != nil {
		return "", errors.WrapPrefix(err, "read Headless/AppVersion", 0)
	}

	return strings.TrimSpace(string(data)), nil
}

// BuildParams は 1 回のビルド invocation の入力.
type BuildParams struct {
	ManifestID  string
	Branch      entity.ResoniteVersionBranch
	GameVersion *string // versions.json 由来. nil の場合はビルド後に Build.version から読む.
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

	if err := b.EnsureRepo(ctx); err != nil {
		return nil, errors.WrapPrefix(err, "ensure container repo", 0)
	}

	appVersion, err := b.readAppVersion()
	if err != nil {
		return nil, err
	}

	slog.Info("image_builder: starting build", "manifest_id", p.ManifestID, "branch", p.Branch, "app_version", appVersion)

	if err := b.downloadResonite(ctx, p); err != nil {
		return nil, errors.WrapPrefix(err, "download resonite", 0)
	}

	resoVersion := ""
	if p.GameVersion != nil {
		resoVersion = *p.GameVersion
	}

	if resoVersion == "" {
		v, err := b.readBuildVersion()
		if err != nil {
			return nil, errors.WrapPrefix(err, "read Build.version", 0)
		}

		resoVersion = v
	}

	if err := b.prepareNativeLibs(); err != nil {
		return nil, errors.WrapPrefix(err, "prepare native-libs", 0)
	}

	tag := computeTag(p.Branch, resoVersion, appVersion)
	ref := fmt.Sprintf("%s:%s", b.dockerCfg.HeadlessImageName, tag)

	if err := runCmd(ctx, b.repoPath, "docker", "build", "-t", ref, "."); err != nil {
		return nil, errors.WrapPrefix(err, "docker build", 0)
	}

	slog.Info("image_builder: build succeeded", "image", ref)

	return &BuildResult{
		ImageTag:        tag,
		ImageRef:        ref,
		ResoniteVersion: resoVersion,
		AppVersion:      appVersion,
	}, nil
}

func (b *Builder) validateCreds() error {
	if b.cfg.SteamUsername == "" || b.cfg.SteamPassword == "" || b.cfg.HeadlessPassword == "" {
		return errors.New("STEAM_USERNAME / STEAM_PASSWORD / HEADLESS_PASSWORD are required")
	}

	return nil
}

// downloadResonite は DepotDownloader を用いて指定 manifest の Resonite headless を取得する.
// container repo の Resonite/ ディレクトリに書き込む.
func (b *Builder) downloadResonite(ctx context.Context, p BuildParams) error {
	// 破壊的な cleanup は DepotDownloader の準備が済んでから. ensureDepotDownloader が
	// 失敗した場合に前回の Resonite/ を残しておいて調査可能にする.
	ddPath, err := b.ensureDepotDownloader(ctx)
	if err != nil {
		return err
	}

	resoDir := filepath.Join(b.repoPath, "Resonite")

	if err := os.RemoveAll(resoDir); err != nil {
		return errors.WrapPrefix(err, "clean Resonite dir", 0)
	}

	if err := os.MkdirAll(resoDir, 0o755); err != nil {
		return errors.WrapPrefix(err, "mkdir Resonite dir", 0)
	}

	filelist := filepath.Join(b.repoPath, "depot-dl-list.txt")

	args := []string{
		"-app", b.cfg.AppID,
		"-manifest", p.ManifestID,
		"-username", b.cfg.SteamUsername,
		"-password", b.cfg.SteamPassword,
		"-dir", resoDir,
		"-os", "linux",
		"-filelist", filelist,
	}

	if b.cfg.HeadlessDepotID != "" {
		args = append(args, "-depot", b.cfg.HeadlessDepotID)
	}

	switch p.Branch {
	case entity.ResoniteVersionBranch_Prerelease:
		args = append(args, "-beta", "prerelease")
	case entity.ResoniteVersionBranch_Headless:
		args = append(args, "-beta", "headless", "-betapassword", b.cfg.HeadlessPassword)
	}

	if err := runCmd(ctx, b.repoPath, ddPath, args...); err != nil {
		return errors.WrapPrefix(err, "DepotDownloader", 0)
	}

	entries, err := os.ReadDir(filepath.Join(resoDir, "Headless"))
	if err != nil || len(entries) == 0 {
		return errors.Errorf("DepotDownloader did not populate Resonite/Headless")
	}

	return nil
}

// ensureDepotDownloader は DepotDownloader バイナリを解決する.
// PATH 上にあればそれを使う (controller の Docker イメージには焼き込み済み).
// 無ければ container repo/bin を確認し、それも無ければ GitHub Releases から
// latest を落とす (ローカル開発用のフォールバック).
func (b *Builder) ensureDepotDownloader(ctx context.Context) (string, error) {
	if p, err := exec.LookPath("DepotDownloader"); err == nil {
		return p, nil
	}

	binDir := filepath.Join(b.repoPath, "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", errors.WrapPrefix(err, "mkdir bin", 0)
	}

	binPath := filepath.Join(binDir, "DepotDownloader")

	if _, err := os.Stat(binPath); err == nil {
		return binPath, nil
	}

	depotOS := "linux"

	arch := "x64"
	if runtime.GOARCH == "arm64" {
		arch = "arm64"
	}

	url := fmt.Sprintf("https://github.com/SteamRE/DepotDownloader/releases/latest/download/DepotDownloader-%s-%s.zip", depotOS, arch)
	zipPath := filepath.Join(binDir, "DepotDownloader.zip")

	// wget / unzip は絶対パスを渡すので cmd.Dir は不要.
	if err := runCmd(ctx, "", "wget", "-q", "-O", zipPath, url); err != nil {
		return "", errors.WrapPrefix(err, "download DepotDownloader", 0)
	}

	if err := runCmd(ctx, "", "unzip", "-o", zipPath, "-d", binDir); err != nil {
		return "", errors.WrapPrefix(err, "unzip DepotDownloader", 0)
	}

	if err := os.Chmod(binPath, 0o755); err != nil {
		return "", errors.WrapPrefix(err, "chmod DepotDownloader", 0)
	}

	_ = os.Remove(zipPath)

	return binPath, nil
}

// prepareNativeLibs は download-resonite.sh の後半と同等の native-libs 整形を Go で行う.
// container repo の Dockerfile が ./native-libs/${TARGETARCH}/* を要求するため.
func (b *Builder) prepareNativeLibs() error {
	root := b.repoPath

	for _, arch := range []struct {
		Name     string
		RuntimeD string
	}{
		{"amd64", "linux-x64"},
		{"arm64", "linux-arm64"},
	} {
		dst := filepath.Join(root, "native-libs", arch.Name)

		if err := os.RemoveAll(dst); err != nil {
			return errors.WrapPrefix(err, "clean native-libs", 0)
		}

		if err := os.MkdirAll(dst, 0o755); err != nil {
			return errors.WrapPrefix(err, "mkdir native-libs", 0)
		}

		runtimeRoot := filepath.Join(root, "Resonite", "Headless", "runtimes", arch.RuntimeD)

		for _, sub := range []string{"lib", "native"} {
			base := filepath.Join(runtimeRoot, sub)
			// lib 側は net<ver> 等の中間ディレクトリを持つので walk.
			// 前提: `base` が丸ごと存在しないのは正常 (headless の linux ビルドは
			// arm64 runtime を欠くことがある); それ以外の I/O エラーは build を止める.
			if _, statErr := os.Stat(base); os.IsNotExist(statErr) {
				continue
			}

			if err := copyFlatFiles(base, dst); err != nil {
				return errors.WrapPrefix(err, fmt.Sprintf("copy native-libs %s/%s", arch.Name, sub), 0)
			}
		}

		if arch.Name == "amd64" {
			// download-resonite.sh L124: amd64 側の libbrolib.so は死蔵ファイルなので削除
			_ = os.Remove(filepath.Join(dst, "libbrolib.so"))
		}
	}

	return nil
}

func copyFlatFiles(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			return nil
		}

		out := filepath.Join(dst, filepath.Base(p))

		return copyFile(p, out)
	})
}

func copyFile(src, dst string) error {
	sf, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = sf.Close() }()

	df, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = df.Close() }()

	_, err = io.Copy(df, sf)

	return err
}

func (b *Builder) readBuildVersion() (string, error) {
	p := filepath.Join(b.repoPath, "Resonite", "Headless", "Build.version")

	data, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// computeTag は既存の parseTag と互換のあるタグ文字列を組み立てる.
// `<[prerelease-]resoniteVersion>-<appVersion>`.
func computeTag(branch entity.ResoniteVersionBranch, resoniteVersion, appVersion string) string {
	prefix := ""
	if branch == entity.ResoniteVersionBranch_Prerelease {
		prefix = "prerelease-"
	}

	return fmt.Sprintf("%s%s-%s", prefix, resoniteVersion, appVersion)
}

// runCmd は cmd を実行し、stdout/stderr を controller ログに流す.
// 失敗時は cmd 名と ExitError の型付きラップを返し、stderr 全文は controller ログに
// 転送するだけで戻り値には含めない (DepotDownloader の stderr に credentials が
// エコーされうるため、上位で DB の build_error に保存されると漏洩する).
func runCmd(ctx context.Context, dir, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var stderr bytes.Buffer

	cmd.Stdout = os.Stdout
	cmd.Stderr = &stderr

	slog.Debug("image_builder: running command", "cmd", name, "dir", dir)

	runErr := cmd.Run()

	// 成功・失敗どちらも stderr は controller ログに出す (build 進捗が stderr に来る).
	if s := stderr.String(); s != "" {
		if _, err := io.Copy(os.Stderr, strings.NewReader(s)); err != nil {
			slog.Debug("failed to relay stderr", "err", err)
		}
	}

	if runErr != nil {
		// %w で ExitError を保持し、errors.Is/As による判定を可能にする.
		// 呼び出し側の SetFailed 経由で DB に流れうるので stderr は含めない.
		return fmt.Errorf("%s failed: %w", name, runErr)
	}

	return nil
}

