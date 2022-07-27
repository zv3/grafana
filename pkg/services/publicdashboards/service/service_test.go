package service

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	dashboardsDB "github.com/grafana/grafana/pkg/services/dashboards/database"
	. "github.com/grafana/grafana/pkg/services/publicdashboards"
	database "github.com/grafana/grafana/pkg/services/publicdashboards/database"
	. "github.com/grafana/grafana/pkg/services/publicdashboards/models"
	"github.com/grafana/grafana/pkg/services/sqlstore"
)

var timeSettings, _ = simplejson.NewJson([]byte(`{"from": "now-12", "to": "now"}`))
var defaultPubdashTimeSettings, _ = simplejson.NewJson([]byte(`{}`))
var dashboardData = simplejson.NewFromAny(map[string]interface{}{"time": map[string]interface{}{"from": "now-8", "to": "now"}})
var mergedDashboardData = simplejson.NewFromAny(map[string]interface{}{"time": map[string]interface{}{"from": "now-12", "to": "now"}})

func TestGetPublicDashboard(t *testing.T) {
	type storeResp struct {
		pd  *PublicDashboard
		d   *models.Dashboard
		err error
	}

	testCases := []struct {
		Name        string
		AccessToken string
		StoreResp   *storeResp
		ErrResp     error
		DashResp    *models.Dashboard
	}{
		{
			Name:        "returns a dashboard",
			AccessToken: "abc123",
			StoreResp: &storeResp{
				pd:  &PublicDashboard{IsEnabled: true},
				d:   &models.Dashboard{Uid: "mydashboard", Data: dashboardData},
				err: nil,
			},
			ErrResp:  nil,
			DashResp: &models.Dashboard{Uid: "mydashboard", Data: dashboardData},
		},
		{
			Name:        "puts pubdash time settings into dashboard",
			AccessToken: "abc123",
			StoreResp: &storeResp{
				pd:  &PublicDashboard{IsEnabled: true, TimeSettings: timeSettings},
				d:   &models.Dashboard{Data: dashboardData},
				err: nil,
			},
			ErrResp:  nil,
			DashResp: &models.Dashboard{Data: mergedDashboardData},
		},
		{
			Name:        "returns ErrPublicDashboardNotFound when isEnabled is false",
			AccessToken: "abc123",
			StoreResp: &storeResp{
				pd:  &PublicDashboard{IsEnabled: false},
				d:   &models.Dashboard{Uid: "mydashboard"},
				err: nil,
			},
			ErrResp:  ErrPublicDashboardNotFound,
			DashResp: nil,
		},
		{
			Name:        "returns ErrPublicDashboardNotFound if PublicDashboard missing",
			AccessToken: "abc123",
			StoreResp:   &storeResp{pd: nil, d: nil, err: nil},
			ErrResp:     ErrPublicDashboardNotFound,
			DashResp:    nil,
		},
		{
			Name:        "returns ErrPublicDashboardNotFound if Dashboard missing",
			AccessToken: "abc123",
			StoreResp:   &storeResp{pd: nil, d: nil, err: nil},
			ErrResp:     ErrPublicDashboardNotFound,
			DashResp:    nil,
		},
	}

	for _, test := range testCases {
		t.Run(test.Name, func(t *testing.T) {
			fakeStore := FakePublicDashboardStore{}
			service := &PublicDashboardServiceImpl{
				log:   log.New("test.logger"),
				store: &fakeStore,
			}

			fakeStore.On("GetPublicDashboard", mock.Anything, mock.Anything).
				Return(test.StoreResp.pd, test.StoreResp.d, test.StoreResp.err)

			dashboard, err := service.GetPublicDashboard(context.Background(), test.AccessToken)
			if test.ErrResp != nil {
				assert.Error(t, test.ErrResp, err)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, test.DashResp, dashboard)

			if test.DashResp != nil {
				assert.NotNil(t, dashboard.CreatedBy)
			}
		})
	}
}

func TestSavePublicDashboard(t *testing.T) {
	t.Run("Saving public dashboard", func(t *testing.T) {
		sqlStore := sqlstore.InitTestDB(t)
		dashboardStore := dashboardsDB.ProvideDashboardStore(sqlStore)
		publicdashboardStore := database.ProvideStore(sqlStore)
		dashboard := insertTestDashboard(t, dashboardStore, "testDashie", 1, 0, true, []map[string]interface{}{})

		service := &PublicDashboardServiceImpl{
			log:   log.New("test.logger"),
			store: publicdashboardStore,
		}

		dto := &SavePublicDashboardConfigDTO{
			DashboardUid: dashboard.Uid,
			OrgId:        dashboard.OrgId,
			UserId:       7,
			PublicDashboard: &PublicDashboard{
				IsEnabled:    true,
				DashboardUid: "NOTTHESAME",
				OrgId:        9999999,
				TimeSettings: timeSettings,
			},
		}

		_, err := service.SavePublicDashboardConfig(context.Background(), dto)
		require.NoError(t, err)

		pubdash, err := service.GetPublicDashboardConfig(context.Background(), dashboard.OrgId, dashboard.Uid)
		require.NoError(t, err)

		// DashboardUid/OrgId/CreatedBy set by the command, not parameters
		assert.Equal(t, dashboard.Uid, pubdash.DashboardUid)
		assert.Equal(t, dashboard.OrgId, pubdash.OrgId)
		assert.Equal(t, dto.UserId, pubdash.CreatedBy)
		// IsEnabled set by parameters
		assert.Equal(t, dto.PublicDashboard.IsEnabled, pubdash.IsEnabled)
		// CreatedAt set to non-zero time
		assert.NotEqual(t, &time.Time{}, pubdash.CreatedAt)
		// Time settings set by db
		assert.Equal(t, timeSettings, pubdash.TimeSettings)
		// accessToken is valid uuid
		_, err = uuid.Parse(pubdash.AccessToken)
		require.NoError(t, err, "expected a valid UUID, got %s", pubdash.AccessToken)
	})

	t.Run("Validate pubdash has default time setting value", func(t *testing.T) {
		sqlStore := sqlstore.InitTestDB(t)
		dashboardStore := dashboardsDB.ProvideDashboardStore(sqlStore)
		publicdashboardStore := database.ProvideStore(sqlStore)
		dashboard := insertTestDashboard(t, dashboardStore, "testDashie", 1, 0, true, []map[string]interface{}{})

		service := &PublicDashboardServiceImpl{
			log:   log.New("test.logger"),
			store: publicdashboardStore,
		}

		dto := &SavePublicDashboardConfigDTO{
			DashboardUid: dashboard.Uid,
			OrgId:        dashboard.OrgId,
			UserId:       7,
			PublicDashboard: &PublicDashboard{
				IsEnabled:    true,
				DashboardUid: "NOTTHESAME",
				OrgId:        9999999,
			},
		}

		_, err := service.SavePublicDashboardConfig(context.Background(), dto)
		require.NoError(t, err)

		pubdash, err := service.GetPublicDashboardConfig(context.Background(), dashboard.OrgId, dashboard.Uid)
		require.NoError(t, err)
		assert.Equal(t, defaultPubdashTimeSettings, pubdash.TimeSettings)
	})

	t.Run("Validate pubdash whose dashboard has template variables returns error", func(t *testing.T) {
		sqlStore := sqlstore.InitTestDB(t)
		dashboardStore := dashboardsDB.ProvideDashboardStore(sqlStore)
		publicdashboardStore := database.ProvideStore(sqlStore)
		templateVars := make([]map[string]interface{}, 1)
		dashboard := insertTestDashboard(t, dashboardStore, "testDashie", 1, 0, true, templateVars)

		service := &PublicDashboardServiceImpl{
			log:   log.New("test.logger"),
			store: publicdashboardStore,
		}

		dto := &SavePublicDashboardConfigDTO{
			DashboardUid: dashboard.Uid,
			OrgId:        dashboard.OrgId,
			UserId:       7,
			PublicDashboard: &PublicDashboard{
				IsEnabled:    true,
				DashboardUid: "NOTTHESAME",
				OrgId:        9999999,
			},
		}

		_, err := service.SavePublicDashboardConfig(context.Background(), dto)
		require.Error(t, err)
	})
}

func TestUpdatePublicDashboard(t *testing.T) {
	t.Run("Updating public dashboard", func(t *testing.T) {
		sqlStore := sqlstore.InitTestDB(t)
		dashboardStore := dashboardsDB.ProvideDashboardStore(sqlStore)
		publicdashboardStore := database.ProvideStore(sqlStore)
		dashboard := insertTestDashboard(t, dashboardStore, "testDashie", 1, 0, true, []map[string]interface{}{})

		service := &PublicDashboardServiceImpl{
			log:   log.New("test.logger"),
			store: publicdashboardStore,
		}

		dto := &SavePublicDashboardConfigDTO{
			DashboardUid: dashboard.Uid,
			OrgId:        dashboard.OrgId,
			UserId:       7,
			PublicDashboard: &PublicDashboard{
				IsEnabled:    true,
				TimeSettings: timeSettings,
			},
		}

		_, err := service.SavePublicDashboardConfig(context.Background(), dto)
		require.NoError(t, err)

		savedPubdash, err := service.GetPublicDashboardConfig(context.Background(), dashboard.OrgId, dashboard.Uid)
		require.NoError(t, err)

		// attempt to overwrite settings
		dto = &SavePublicDashboardConfigDTO{
			DashboardUid: dashboard.Uid,
			OrgId:        dashboard.OrgId,
			UserId:       8,
			PublicDashboard: &PublicDashboard{
				Uid:          savedPubdash.Uid,
				OrgId:        9,
				DashboardUid: "abc1234",
				CreatedBy:    9,
				CreatedAt:    time.Time{},

				IsEnabled:    true,
				TimeSettings: timeSettings,
				AccessToken:  "NOTAREALUUID",
			},
		}

		// Since the dto.PublicDashboard has a uid, this will call
		// service.updatePublicDashboardConfig
		_, err = service.SavePublicDashboardConfig(context.Background(), dto)
		require.NoError(t, err)

		updatedPubdash, err := service.GetPublicDashboardConfig(context.Background(), dashboard.OrgId, dashboard.Uid)
		require.NoError(t, err)

		// don't get updated
		assert.Equal(t, savedPubdash.DashboardUid, updatedPubdash.DashboardUid)
		assert.Equal(t, savedPubdash.OrgId, updatedPubdash.OrgId)
		assert.Equal(t, savedPubdash.CreatedAt, updatedPubdash.CreatedAt)
		assert.Equal(t, savedPubdash.CreatedBy, updatedPubdash.CreatedBy)
		assert.Equal(t, savedPubdash.AccessToken, updatedPubdash.AccessToken)

		// gets updated
		assert.Equal(t, dto.PublicDashboard.IsEnabled, updatedPubdash.IsEnabled)
		assert.Equal(t, dto.PublicDashboard.TimeSettings, updatedPubdash.TimeSettings)
		assert.Equal(t, dto.UserId, updatedPubdash.UpdatedBy)
		assert.NotEqual(t, &time.Time{}, updatedPubdash.UpdatedAt)
	})

	t.Run("Updating set empty time settings", func(t *testing.T) {
		sqlStore := sqlstore.InitTestDB(t)
		dashboardStore := dashboardsDB.ProvideDashboardStore(sqlStore)
		publicdashboardStore := database.ProvideStore(sqlStore)
		dashboard := insertTestDashboard(t, dashboardStore, "testDashie", 1, 0, true, []map[string]interface{}{})

		service := &PublicDashboardServiceImpl{
			log:   log.New("test.logger"),
			store: publicdashboardStore,
		}

		dto := &SavePublicDashboardConfigDTO{
			DashboardUid: dashboard.Uid,
			OrgId:        dashboard.OrgId,
			UserId:       7,
			PublicDashboard: &PublicDashboard{
				IsEnabled:    true,
				TimeSettings: timeSettings,
			},
		}

		// Since the dto.PublicDashboard has a uid, this will call
		// service.updatePublicDashboardConfig
		_, err := service.SavePublicDashboardConfig(context.Background(), dto)
		require.NoError(t, err)

		savedPubdash, err := service.GetPublicDashboardConfig(context.Background(), dashboard.OrgId, dashboard.Uid)
		require.NoError(t, err)

		// attempt to overwrite settings
		dto = &SavePublicDashboardConfigDTO{
			DashboardUid: dashboard.Uid,
			OrgId:        dashboard.OrgId,
			UserId:       8,
			PublicDashboard: &PublicDashboard{
				Uid:          savedPubdash.Uid,
				OrgId:        9,
				DashboardUid: "abc1234",
				CreatedBy:    9,
				CreatedAt:    time.Time{},

				IsEnabled:   true,
				AccessToken: "NOTAREALUUID",
			},
		}

		_, err = service.SavePublicDashboardConfig(context.Background(), dto)
		require.NoError(t, err)

		updatedPubdash, err := service.GetPublicDashboardConfig(context.Background(), dashboard.OrgId, dashboard.Uid)
		require.NoError(t, err)

		timeSettings, err := simplejson.NewJson([]byte("{}"))
		require.NoError(t, err)

		assert.Equal(t, timeSettings, updatedPubdash.TimeSettings)
	})
}

func TestBuildAnonymousUser(t *testing.T) {
	sqlStore := sqlstore.InitTestDB(t)
	dashboardStore := dashboardsDB.ProvideDashboardStore(sqlStore)
	dashboard := insertTestDashboard(t, dashboardStore, "testDashie", 1, 0, true, []map[string]interface{}{})
	publicdashboardStore := database.ProvideStore(sqlStore)
	service := &PublicDashboardServiceImpl{
		log:   log.New("test.logger"),
		store: publicdashboardStore,
	}

	t.Run("will add datasource read and query permissions to user for each datasource in dashboard", func(t *testing.T) {
		user, err := service.BuildAnonymousUser(context.Background(), dashboard)
		require.NoError(t, err)
		require.Equal(t, dashboard.OrgId, user.OrgId)
		require.Equal(t, "datasources:uid:ds1", user.Permissions[user.OrgId]["datasources:query"][0])
		require.Equal(t, "datasources:uid:ds3", user.Permissions[user.OrgId]["datasources:query"][1])
		require.Equal(t, "datasources:uid:ds1", user.Permissions[user.OrgId]["datasources:read"][0])
		require.Equal(t, "datasources:uid:ds3", user.Permissions[user.OrgId]["datasources:read"][1])
	})
}

func TestBuildPublicDashboardMetricRequest(t *testing.T) {
	sqlStore := sqlstore.InitTestDB(t)
	dashboardStore := dashboardsDB.ProvideDashboardStore(sqlStore)
	publicdashboardStore := database.ProvideStore(sqlStore)

	publicDashboard := insertTestDashboard(t, dashboardStore, "testDashie", 1, 0, true, []map[string]interface{}{})
	nonPublicDashboard := insertTestDashboard(t, dashboardStore, "testNonPublicDashie", 1, 0, true, []map[string]interface{}{})

	service := &PublicDashboardServiceImpl{
		log:   log.New("test.logger"),
		store: publicdashboardStore,
	}

	dto := &SavePublicDashboardConfigDTO{
		DashboardUid: publicDashboard.Uid,
		OrgId:        publicDashboard.OrgId,
		PublicDashboard: &PublicDashboard{
			IsEnabled:    true,
			DashboardUid: "NOTTHESAME",
			OrgId:        9999999,
			TimeSettings: timeSettings,
		},
	}

	publicDashboardPD, err := service.SavePublicDashboardConfig(context.Background(), dto)
	require.NoError(t, err)

	nonPublicDto := &SavePublicDashboardConfigDTO{
		DashboardUid: nonPublicDashboard.Uid,
		OrgId:        nonPublicDashboard.OrgId,
		PublicDashboard: &PublicDashboard{
			IsEnabled:    false,
			DashboardUid: "NOTTHESAME",
			OrgId:        9999999,
			TimeSettings: defaultPubdashTimeSettings,
		},
	}

	nonPublicDashboardPD, err := service.SavePublicDashboardConfig(context.Background(), nonPublicDto)
	require.NoError(t, err)

	t.Run("extracts queries from provided dashboard", func(t *testing.T) {
		reqDTO, err := service.BuildPublicDashboardMetricRequest(
			context.Background(),
			publicDashboard,
			publicDashboardPD,
			1,
		)
		require.NoError(t, err)

		require.Equal(t, timeSettings.Get("from").MustString(), reqDTO.From)
		require.Equal(t, timeSettings.Get("to").MustString(), reqDTO.To)
		require.Len(t, reqDTO.Queries, 2)
		require.Equal(
			t,
			simplejson.MustJson([]byte(`{
				"datasource": {
					"type": "mysql",
					"uid": "ds1"
				},
				"refId": "A"
			}`)),
			reqDTO.Queries[0],
		)
		require.Equal(
			t,
			simplejson.MustJson([]byte(`{
				"datasource": {
					"type": "prometheus",
					"uid": "ds2"
				},
				"refId": "B"
			}`)),
			reqDTO.Queries[1],
		)
	})

	t.Run("returns an error when panel missing", func(t *testing.T) {
		_, err := service.BuildPublicDashboardMetricRequest(
			context.Background(),
			publicDashboard,
			publicDashboardPD,
			49,
		)

		require.ErrorContains(t, err, "Panel not found")
	})

	t.Run("returns an error when dashboard not public", func(t *testing.T) {
		_, err := service.BuildPublicDashboardMetricRequest(
			context.Background(),
			nonPublicDashboard,
			nonPublicDashboardPD,
			2,
		)
		require.ErrorContains(t, err, "Public dashboard not found")
	})
}

func insertTestDashboard(t *testing.T, dashboardStore *dashboardsDB.DashboardStore, title string, orgId int64,
	folderId int64, isFolder bool, templateVars []map[string]interface{}, tags ...interface{}) *models.Dashboard {
	t.Helper()
	cmd := models.SaveDashboardCommand{
		OrgId:    orgId,
		FolderId: folderId,
		IsFolder: isFolder,
		Dashboard: simplejson.NewFromAny(map[string]interface{}{
			"id":    nil,
			"title": title,
			"tags":  tags,
			"panels": []interface{}{
				map[string]interface{}{
					"id": 1,
					"datasource": map[string]interface{}{
						"uid": "ds1",
					},
					"targets": []interface{}{
						map[string]interface{}{
							"datasource": map[string]interface{}{
								"type": "mysql",
								"uid":  "ds1",
							},
							"refId": "A",
						},
						map[string]interface{}{
							"datasource": map[string]interface{}{
								"type": "prometheus",
								"uid":  "ds2",
							},
							"refId": "B",
						},
					},
				},
				map[string]interface{}{
					"id": 2,
					"datasource": map[string]interface{}{
						"uid": "ds3",
					},
					"targets": []interface{}{
						map[string]interface{}{
							"datasource": map[string]interface{}{
								"type": "mysql",
								"uid":  "ds3",
							},
							"refId": "C",
						},
					},
				},
			},
			"templating": map[string]interface{}{
				"list": templateVars,
			},
		}),
	}
	dash, err := dashboardStore.SaveDashboard(cmd)
	require.NoError(t, err)
	require.NotNil(t, dash)
	dash.Data.Set("id", dash.Id)
	dash.Data.Set("uid", dash.Uid)
	return dash
}
