package kvstore

import (
	"context"
	"errors"
	"testing"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/plugins/backendplugin/secretsmanagerplugin"
	"github.com/grafana/grafana/pkg/services/featuremgmt"
	"github.com/grafana/grafana/pkg/services/secrets/database"
	"github.com/grafana/grafana/pkg/services/secrets/manager"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"google.golang.org/grpc"
)

func SetupTestService(t *testing.T) SecretsKVStore {
	t.Helper()

	sqlStore := sqlstore.InitTestDB(t)
	store := database.ProvideSecretsStore(sqlstore.InitTestDB(t))
	secretsService := manager.SetupTestService(t, store)

	kv := &secretsKVStoreSQL{
		sqlStore:       sqlStore,
		log:            log.New("secrets.kvstore"),
		secretsService: secretsService,
		decryptionCache: decryptionCache{
			cache: make(map[int64]cachedDecrypted),
		},
	}

	return kv
}

// In memory kv store used for testing
type FakeSecretsKVStore struct {
	store map[Key]string
}

func NewFakeSecretsKVStore() FakeSecretsKVStore {
	return FakeSecretsKVStore{store: make(map[Key]string)}
}

func (f FakeSecretsKVStore) Get(ctx context.Context, orgId int64, namespace string, typ string) (string, bool, error) {
	value := f.store[buildKey(orgId, namespace, typ)]
	found := value != ""
	return value, found, nil
}

func (f FakeSecretsKVStore) Set(ctx context.Context, orgId int64, namespace string, typ string, value string) error {
	f.store[buildKey(orgId, namespace, typ)] = value
	return nil
}

func (f FakeSecretsKVStore) Del(ctx context.Context, orgId int64, namespace string, typ string) error {
	delete(f.store, buildKey(orgId, namespace, typ))
	return nil
}

func (f FakeSecretsKVStore) Keys(ctx context.Context, orgId int64, namespace string, typ string) ([]Key, error) {
	res := make([]Key, 0)
	for k := range f.store {
		if k.OrgId == orgId && k.Namespace == namespace && k.Type == typ {
			res = append(res, k)
		}
	}
	return res, nil
}

func (f FakeSecretsKVStore) Rename(ctx context.Context, orgId int64, namespace string, typ string, newNamespace string) error {
	f.store[buildKey(orgId, newNamespace, typ)] = f.store[buildKey(orgId, namespace, typ)]
	delete(f.store, buildKey(orgId, namespace, typ))
	return nil
}

func buildKey(orgId int64, namespace string, typ string) Key {
	return Key{
		OrgId:     orgId,
		Namespace: namespace,
		Type:      typ,
	}
}

// Fake feature toggle - only need to check the backwards compatibility disabled flag
type fakeFeatureToggles struct {
	returnValue bool
}

func NewFakeFeatureToggles(t *testing.T, returnValue bool) featuremgmt.FeatureToggles {
	t.Helper()
	return fakeFeatureToggles{
		returnValue: returnValue,
	}
}

func (f fakeFeatureToggles) IsEnabled(feature string) bool {
	return f.returnValue
}

// Fake Remote Secrets Plugin Check
type mockRemoteSecretsPluginCheck struct {
	UseRemoteSecretsPluginCheck
	shouldReturnError bool
}

func provideMockRemotePluginCheck() *mockRemoteSecretsPluginCheck {
	return &mockRemoteSecretsPluginCheck{}
}

func provideMockRemotePluginCheckWithErr() *mockRemoteSecretsPluginCheck {
	return &mockRemoteSecretsPluginCheck{
		shouldReturnError: true,
	}
}

func (c *mockRemoteSecretsPluginCheck) ShouldUseRemoteSecretsPlugin() bool {
	return true
}

func (c *mockRemoteSecretsPluginCheck) StartAndReturnPlugin(ctx context.Context) (secretsmanagerplugin.SecretsManagerPlugin, error) {
	var err error
	if c.shouldReturnError {
		err = errors.New("bogus")
	}
	return &fakeGRPCSecretsPlugin{}, err
}

// Fake grpc secrets plugin impl
type fakeGRPCSecretsPlugin struct{}

func (c *fakeGRPCSecretsPlugin) GetSecret(ctx context.Context, in *secretsmanagerplugin.GetSecretRequest, opts ...grpc.CallOption) (*secretsmanagerplugin.GetSecretResponse, error) {
	return &secretsmanagerplugin.GetSecretResponse{
		DecryptedValue: "bogus",
		Exists:         true,
	}, nil
}

func (c *fakeGRPCSecretsPlugin) SetSecret(ctx context.Context, in *secretsmanagerplugin.SetSecretRequest, opts ...grpc.CallOption) (*secretsmanagerplugin.SetSecretResponse, error) {
	return &secretsmanagerplugin.SetSecretResponse{}, nil
}

func (c *fakeGRPCSecretsPlugin) DeleteSecret(ctx context.Context, in *secretsmanagerplugin.DeleteSecretRequest, opts ...grpc.CallOption) (*secretsmanagerplugin.DeleteSecretResponse, error) {
	return &secretsmanagerplugin.DeleteSecretResponse{}, nil
}

func (c *fakeGRPCSecretsPlugin) ListSecrets(ctx context.Context, in *secretsmanagerplugin.ListSecretsRequest, opts ...grpc.CallOption) (*secretsmanagerplugin.ListSecretsResponse, error) {
	return &secretsmanagerplugin.ListSecretsResponse{
		Keys: make([]*secretsmanagerplugin.Key, 0),
	}, nil
}

func (c *fakeGRPCSecretsPlugin) RenameSecret(ctx context.Context, in *secretsmanagerplugin.RenameSecretRequest, opts ...grpc.CallOption) (*secretsmanagerplugin.RenameSecretResponse, error) {
	return &secretsmanagerplugin.RenameSecretResponse{}, nil
}

var _ SecretsKVStore = FakeSecretsKVStore{}
var _ secretsmanagerplugin.SecretsManagerPlugin = &fakeGRPCSecretsPlugin{}
