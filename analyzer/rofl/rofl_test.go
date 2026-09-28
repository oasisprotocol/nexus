package rofl

import (
	"context"
	"testing"

	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/modules/rofl"
	sdkTesting "github.com/oasisprotocol/oasis-sdk/client-sdk/go/testing"
	"github.com/oasisprotocol/oasis-sdk/client-sdk/go/types"
	"github.com/stretchr/testify/require"

	"github.com/oasisprotocol/nexus/analyzer/queries"
	"github.com/oasisprotocol/nexus/common"
	"github.com/oasisprotocol/nexus/storage"
	"github.com/oasisprotocol/nexus/storage/oasis/nodeapi"
)

// mockRoflSource returns a fixed app config from RoflApp.
type mockRoflSource struct {
	nodeapi.RuntimeApiLite
	app *nodeapi.AppConfig
}

func (m *mockRoflSource) RoflApp(_ context.Context, _ uint64, _ nodeapi.AppID) (*nodeapi.AppConfig, error) {
	return m.app, nil
}

func TestQueueRoflAppRefreshAdmin(t *testing.T) {
	appID := rofl.NewAppIDGlobalName("test")
	admin := sdkTesting.Alice.Address

	for _, tc := range []struct {
		name          string
		admin         *types.Address
		expectedAdmin *string
	}{
		{"with admin", &admin, common.Ptr(admin.String())},
		{"without admin", nil, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := &processor{
				runtime: common.RuntimeSapphire,
				source: &mockRoflSource{app: &nodeapi.AppConfig{
					ID:    appID,
					Admin: tc.admin,
				}},
			}
			batch := &storage.QueryBatch{}
			require.NoError(t, p.queueRoflAppRefresh(context.Background(), batch, 100, appID))

			items := batch.Queries()
			require.Len(t, items, 1)
			require.Equal(t, queries.RuntimeRoflAppUpdate, items[0].Cmd)
			require.Equal(t, appID.String(), items[0].Args[1])
			require.Equal(t, tc.expectedAdmin, items[0].Args[2])
		})
	}
}
