package adapter

import (
	"context"
	"time"

	"github.com/go-errors/errors"

	"github.com/dchest/uniuri"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/converter"
	"github.com/hantabaru1014/baru-reso-headless-controller/adapter/hostconnector"
	"github.com/hantabaru1014/baru-reso-headless-controller/db"
	"github.com/hantabaru1014/baru-reso-headless-controller/domain/entity"
	headlessv1 "github.com/hantabaru1014/baru-reso-headless-controller/pbgen/headless/v1"
	"github.com/hantabaru1014/baru-reso-headless-controller/usecase/port"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/encoding/protojson"
)

var _ port.HeadlessHostRepository = (*HeadlessHostRepository)(nil)

type HeadlessHostRepository struct {
	q         *db.Queries
	connector hostconnector.HostConnector
}

// UpdateHostSettings implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) UpdateHostSettings(ctx context.Context, id string, settings *entity.HeadlessHostSettings) error {
	json, err := protojson.Marshal(converter.HeadlessHostSettingsToStartupConfigProto(settings))
	if err != nil {
		return errors.Wrap(err, 0)
	}
	return h.q.UpdateHostLastStartupConfig(ctx, db.UpdateHostLastStartupConfigParams{
		ID:                id,
		LastStartupConfig: json,
	})
}

// Find implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) Find(ctx context.Context, id string, fetchOptions port.HeadlessHostFetchOptions) (*entity.HeadlessHost, error) {
	host, err := h.q.GetHost(ctx, id)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}
	return h.dbToEntity(ctx, &host, fetchOptions)
}

// GetLogs implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) GetLogs(ctx context.Context, id string, limit int32, until string, since string) (port.LogLineList, error) {
	host, err := h.q.GetHost(ctx, id)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}
	connector, err := h.getConnector(host.ConnectorType)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return connector.GetLogs(ctx, hostconnector.HostConnectString(host.ConnectString), limit, until, since)
}

// GetRpcClient implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) GetRpcClient(ctx context.Context, id string) (headlessv1.HeadlessControlServiceClient, error) {
	host, err := h.q.GetHost(ctx, id)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}
	connector, err := h.getConnector(host.ConnectorType)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	return connector.GetRpcClient(ctx, hostconnector.HostConnectString(host.ConnectString))
}

// ListAll implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) ListAll(ctx context.Context, fetchOptions port.HeadlessHostFetchOptions) (entity.HeadlessHostList, error) {
	hosts, err := h.q.ListHosts(ctx)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}
	var result entity.HeadlessHostList
	for _, host := range hosts {
		// TODO: getContainerが毎回Listしているので呼び出しを最適化したい
		entityHost, err := h.dbToEntity(ctx, &host, fetchOptions)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
		result = append(result, entityHost)
	}
	return result, nil
}

// ListRunningByAccount implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) ListRunningByAccount(ctx context.Context, accountId string) (entity.HeadlessHostList, error) {
	hosts, err := h.q.ListRunningHostsByAccount(ctx, accountId)
	if err != nil {
		return nil, errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}
	var result entity.HeadlessHostList
	for _, host := range hosts {
		connector, err := h.getConnector(host.ConnectorType)
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
		status := connector.GetStatus(ctx, hostconnector.HostConnectString(host.ConnectString))
		host := &entity.HeadlessHost{
			ID:               host.ID,
			Name:             host.Name,
			AccountId:        host.AccountID,
			Status:           status,
			AutoUpdatePolicy: entity.HostAutoUpdatePolicy(host.AutoUpdatePolicy),
		}
		if status == entity.HeadlessHostStatus_RUNNING {
			result = append(result, host)
		}
	}

	return result, nil
}

// ListContainerTags implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) ListContainerTags(ctx context.Context, lastTag *string) (port.ContainerImageList, error) {
	return h.connector.ListContainerTags(ctx, lastTag)
}

// Rename implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) Rename(ctx context.Context, id string, newName string) error {
	return h.q.UpdateHostName(ctx, db.UpdateHostNameParams{
		ID:   id,
		Name: newName,
	})
}

// Restart implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) Restart(ctx context.Context, id string, newStartupConfig port.HeadlessHostStartParams, timeoutSeconds int) error {
	dbHost, err := h.q.GetHost(ctx, id)
	if err != nil {
		return errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}
	connector, err := h.getConnector(dbHost.ConnectorType)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	connectStr := hostconnector.HostConnectString(dbHost.ConnectString)
	_ = h.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
		ID:     id,
		Status: int32(entity.HeadlessHostStatus_STOPPING),
	})
	status := connector.GetStatus(ctx, connectStr)
	if status == entity.HeadlessHostStatus_RUNNING {
		err = connector.Stop(ctx, connectStr, timeoutSeconds)
		if err != nil {
			return errors.Wrap(err, 0)
		}
	}
	err = h.q.UpdateHostMemo(ctx, db.UpdateHostMemoParams{
		ID: id,
		Memo: pgtype.Text{
			Valid:  true,
			String: newStartupConfig.Memo,
		},
	})
	if err != nil {
		return errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}
	err = h.q.UpdateHostAutoUpdatePolicy(ctx, db.UpdateHostAutoUpdatePolicyParams{
		ID:               id,
		AutoUpdatePolicy: int32(newStartupConfig.AutoUpdatePolicy),
	})
	if err != nil {
		return errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}
	_ = h.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
		ID:     id,
		Status: int32(entity.HeadlessHostStatus_STARTING),
	})
	hostStartParams := hostconnector.HostStartParams{
		ID:                id,
		ContainerImageTag: newStartupConfig.ContainerImageTag,
		HeadlessAccount:   newStartupConfig.HeadlessAccount,
		StartupConfig:     newStartupConfig.StartupConfig,
	}
	newConnectStr, err := connector.Start(ctx, hostStartParams)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	err = h.q.UpdateHostConnectString(ctx, db.UpdateHostConnectStringParams{
		ID:            id,
		ConnectString: string(newConnectStr),
	})
	if err != nil {
		return errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}
	_ = h.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
		ID:     id,
		Status: int32(entity.HeadlessHostStatus_RUNNING),
	})
	json, err := protojson.Marshal(newStartupConfig.StartupConfig)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	err = h.q.UpdateHostLastStartupConfig(ctx, db.UpdateHostLastStartupConfigParams{
		ID:                id,
		LastStartupConfig: json,
	})
	if err != nil {
		return errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}

	return nil
}

// Start implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) Start(ctx context.Context, connector port.HostConnectorType, params port.HeadlessHostStartParams, userId *string) (string, error) {
	connectorImpl, err := h.getConnector(string(connector))
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	json, err := protojson.Marshal(params.StartupConfig)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	id := uniuri.New()
	startParams := hostconnector.HostStartParams{
		ID:                id,
		ContainerImageTag: params.ContainerImageTag,
		HeadlessAccount:   params.HeadlessAccount,
		StartupConfig:     params.StartupConfig,
	}
	newConnectStr, err := connectorImpl.Start(ctx, startParams)
	if err != nil {
		return "", errors.Wrap(err, 0)
	}
	ownerId := pgtype.Text{
		Valid: userId != nil,
	}
	if userId != nil {
		ownerId.String = *userId
	}
	dbHost, err := h.q.CreateHost(ctx, db.CreateHostParams{
		ID:                             id,
		Name:                           params.Name,
		Status:                         int32(entity.HeadlessHostStatus_RUNNING),
		AccountID:                      params.HeadlessAccount.ResoniteID,
		OwnerID:                        ownerId,
		LastStartupConfig:              json,
		LastStartupConfigSchemaVersion: 1,
		ConnectorType:                  string(connector),
		ConnectString:                  string(newConnectStr),
		AutoUpdatePolicy:               int32(params.AutoUpdatePolicy),
		Memo: pgtype.Text{
			Valid:  true,
			String: params.Memo,
		},
		StartedAt: pgtype.Timestamptz{
			Valid: true,
			Time:  time.Now(),
		},
	})
	if err != nil {
		return "", errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}

	return dbHost.ID, nil
}

// Stop implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) Stop(ctx context.Context, id string, timeoutSeconds int) error {
	dbHost, err := h.q.GetHost(ctx, id)
	if err != nil {
		return errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}
	connector, err := h.getConnector(dbHost.ConnectorType)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	_ = h.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
		ID:     dbHost.ID,
		Status: int32(entity.HeadlessHostStatus_STOPPING),
	})
	client, err := connector.GetRpcClient(ctx, hostconnector.HostConnectString(dbHost.ConnectString))
	if err != nil {
		return errors.Wrap(err, 0)
	}
	startupConfig, err := client.GetStartupConfigToRestore(ctx, &headlessv1.GetStartupConfigToRestoreRequest{
		IncludeStartWorlds: true,
	})
	if err != nil {
		return errors.Wrap(err, 0)
	}
	json, err := protojson.Marshal(startupConfig.StartupConfig)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	err = h.q.UpdateHostLastStartupConfig(ctx, db.UpdateHostLastStartupConfigParams{
		ID:                dbHost.ID,
		LastStartupConfig: json,
	})
	if err != nil {
		return errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}

	err = connector.Stop(ctx, hostconnector.HostConnectString(dbHost.ConnectString), timeoutSeconds)
	if err != nil {
		return errors.Wrap(err, 0)
	}

	_ = h.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
		ID:     dbHost.ID,
		Status: int32(entity.HeadlessHostStatus_EXITED),
	})

	return nil
}

// Kill implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) Kill(ctx context.Context, id string) error {
	dbHost, err := h.q.GetHost(ctx, id)
	if err != nil {
		return errors.WrapPrefix(convertDBErr(err), "headless host", 0)
	}
	connector, err := h.getConnector(dbHost.ConnectorType)
	if err != nil {
		return errors.Wrap(err, 0)
	}
	_ = h.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
		ID:     dbHost.ID,
		Status: int32(entity.HeadlessHostStatus_STOPPING),
	})

	err = connector.Kill(ctx, hostconnector.HostConnectString(dbHost.ConnectString))
	if err != nil {
		return errors.Wrap(err, 0)
	}

	_ = h.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
		ID:     dbHost.ID,
		Status: int32(entity.HeadlessHostStatus_EXITED),
	})

	return nil
}

// Delete implements port.HeadlessHostRepository.
func (h *HeadlessHostRepository) Delete(ctx context.Context, id string) error {
	return h.q.DeleteHost(ctx, id)
}

func NewHeadlessHostRepository(q *db.Queries, connector hostconnector.HostConnector) *HeadlessHostRepository {
	return &HeadlessHostRepository{
		q:         q,
		connector: connector,
	}
}

func (h *HeadlessHostRepository) getConnector(connector_type string) (hostconnector.HostConnector, error) {
	switch port.HostConnectorType(connector_type) {
	case port.HostConnectorType_DOCKER:
		return h.connector, nil
	default:
		return nil, errors.New("unsupported connector type: " + connector_type)
	}
}

func (h *HeadlessHostRepository) fetchHostInfo(ctx context.Context, host *entity.HeadlessHost, client headlessv1.HeadlessControlServiceClient) error {
	info, err := client.GetAbout(ctx, &headlessv1.GetAboutRequest{})
	if err != nil {
		return errors.Wrap(err, 0)
	}
	host.ResoniteVersion = info.ResoniteVersion
	host.AppVersion = info.AppVersion

	accountInfo, err := client.GetAccountInfo(ctx, &headlessv1.GetAccountInfoRequest{})
	if err != nil {
		return errors.Wrap(err, 0)
	}
	host.AccountId = accountInfo.UserId
	host.AccountName = accountInfo.DisplayName

	// TODO: ライフタイムが全く別物なのでentityから外す
	status, err := client.GetStatus(ctx, &headlessv1.GetStatusRequest{})
	if err != nil {
		return errors.Wrap(err, 0)
	}
	host.Fps = status.Fps

	return nil
}

func (h *HeadlessHostRepository) dbToEntity(ctx context.Context, dbHost *db.Host, fetchOptions port.HeadlessHostFetchOptions) (*entity.HeadlessHost, error) {
	connector, err := h.getConnector(dbHost.ConnectorType)
	if err != nil {
		return nil, errors.Wrap(err, 0)
	}
	status := connector.GetStatus(ctx, hostconnector.HostConnectString(dbHost.ConnectString))
	host := &entity.HeadlessHost{
		ID:               dbHost.ID,
		Name:             dbHost.Name,
		AccountId:        dbHost.AccountID,
		Status:           entity.HeadlessHostStatus(dbHost.Status),
		AutoUpdatePolicy: entity.HostAutoUpdatePolicy(dbHost.AutoUpdatePolicy),
	}
	if dbHost.Memo.Valid {
		host.Memo = dbHost.Memo.String
	}
	if status == entity.HeadlessHostStatus_RUNNING {
		conn, err := connector.GetRpcClient(ctx, hostconnector.HostConnectString(dbHost.ConnectString))
		if err != nil {
			return nil, errors.Wrap(err, 0)
		}
		err = h.fetchHostInfo(ctx, host, conn)
		if err == nil {
			startupConfig, err := conn.GetStartupConfigToRestore(ctx, &headlessv1.GetStartupConfigToRestoreRequest{
				IncludeStartWorlds: fetchOptions.IncludeStartWorlds,
			})
			if err == nil {
				host.HostSettings = *converter.HeadlessHostSettingsProtoToEntity(startupConfig.StartupConfig)
			}
		}
	} else {
		if dbHost.LastStartupConfig != nil {
			parsed := &headlessv1.StartupConfig{}
			if err := protojson.Unmarshal(dbHost.LastStartupConfig, parsed); err != nil {
				return nil, errors.Wrap(err, 0)
			}
			host.HostSettings = *converter.HeadlessHostSettingsProtoToEntity(parsed)
		}
		account, err := h.q.GetHeadlessAccount(ctx, dbHost.AccountID)
		if err == nil {
			host.AccountName = account.LastDisplayName.String
		}
	}
	if status != entity.HeadlessHostStatus_UNKNOWN {
		host.Status = status
		if dbHost.Status != int32(status) {
			_ = h.q.UpdateHostStatus(ctx, db.UpdateHostStatusParams{
				ID:     dbHost.ID,
				Status: int32(status),
			})
		}
	}

	return host, nil
}
