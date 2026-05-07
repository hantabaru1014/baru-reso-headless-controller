package usecase

import (
	"context"
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"regexp"

	"github.com/go-errors/errors"
	"github.com/google/uuid"
	"github.com/hantabaru1014/baru-reso-headless-controller/lib/blobstore"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
)

type BlobUsecase struct {
	sessionRepo port.SessionRepository
	hostRepo    port.HeadlessHostRepository
	blob        blobstore.Client
}

func NewBlobUsecase(sessionRepo port.SessionRepository, hostRepo port.HeadlessHostRepository, blob blobstore.Client) *BlobUsecase {
	return &BlobUsecase{
		sessionRepo: sessionRepo,
		hostRepo:    hostRepo,
		blob:        blob,
	}
}

// PrepareSessionWorldDownload streams the session's world from the headless
// container into a local temp file, uploads it to the blob store under a
// fresh UUID, and returns the relative download URL plus suggested filename.
func (u *BlobUsecase) PrepareSessionWorldDownload(ctx context.Context, sessionID string, format headlessv1.WorldBinaryFormat) (string, string, error) {
	dbSession, err := u.sessionRepo.Get(ctx, sessionID)
	if err != nil {
		return "", "", errors.Wrap(err, 0)
	}

	client, err := u.hostRepo.GetRpcClient(ctx, dbSession.HostID)
	if err != nil {
		return "", "", errors.Wrap(err, 0)
	}

	includeVariants := false

	stream, err := client.DownloadSessionWorld(ctx, &headlessv1.DownloadSessionWorldRequest{
		SessionId:       sessionID,
		Format:          format,
		IncludeVariants: &includeVariants,
	})
	if err != nil {
		return "", "", errors.Wrap(err, 0)
	}

	tmpFile, err := os.CreateTemp("", "world-download-*")
	if err != nil {
		return "", "", errors.Wrap(err, 0)
	}

	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpFile.Name())
	}()

	for {
		chunk, recvErr := stream.Recv()
		if stderrors.Is(recvErr, io.EOF) {
			break
		}

		if recvErr != nil {
			return "", "", errors.Wrap(recvErr, 0)
		}

		if _, err := tmpFile.Write(chunk.GetChunk()); err != nil {
			return "", "", errors.Wrap(err, 0)
		}
	}

	stat, err := tmpFile.Stat()
	if err != nil {
		return "", "", errors.Wrap(err, 0)
	}

	if _, err := tmpFile.Seek(0, io.SeekStart); err != nil {
		return "", "", errors.Wrap(err, 0)
	}

	displayName := dbSession.Name
	if cs := dbSession.CurrentState; cs != nil && cs.GetName() != "" {
		displayName = cs.GetName()
	}

	if displayName == "" {
		displayName = sessionID
	}

	filename := fmt.Sprintf("%s.%s", sanitizeFilename(displayName), worldBinaryFormatExtension(format))
	key := uuid.NewString()

	if err := u.blob.Upload(ctx, key, tmpFile, stat.Size(), filename, "application/octet-stream"); err != nil {
		slog.Error("failed to upload world download blob", "error", err, "sessionId", sessionID)
		return "", "", errors.Wrap(err, 0)
	}

	return "/blobs/" + key, filename, nil
}

var unsafeFilenameChars = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]`)

func sanitizeFilename(name string) string {
	cleaned := unsafeFilenameChars.ReplaceAllString(name, "_")
	if cleaned == "" {
		return "world"
	}

	return cleaned
}

func worldBinaryFormatExtension(f headlessv1.WorldBinaryFormat) string {
	switch f {
	case headlessv1.WorldBinaryFormat_WORLD_BINARY_FORMAT_7ZBSON:
		return "7zbson"
	case headlessv1.WorldBinaryFormat_WORLD_BINARY_FORMAT_BRSON:
		return "brson"
	case headlessv1.WorldBinaryFormat_WORLD_BINARY_FORMAT_RESONITEPACKAGE:
		return "resonitepackage"
	default:
		return "bin"
	}
}
