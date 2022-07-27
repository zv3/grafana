package provisioning

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/grafana/grafana/pkg/infra/log"
	plugifaces "github.com/grafana/grafana/pkg/plugins"
	"github.com/grafana/grafana/pkg/registry"
	"github.com/grafana/grafana/pkg/services/accesscontrol"
	"github.com/grafana/grafana/pkg/services/alerting"
	"github.com/grafana/grafana/pkg/services/correlations"
	dashboardservice "github.com/grafana/grafana/pkg/services/dashboards"
	datasourceservice "github.com/grafana/grafana/pkg/services/datasources"
	"github.com/grafana/grafana/pkg/services/encryption"
	"github.com/grafana/grafana/pkg/services/ngalert/provisioning"
	"github.com/grafana/grafana/pkg/services/ngalert/store"
	"github.com/grafana/grafana/pkg/services/notifications"
	"github.com/grafana/grafana/pkg/services/pluginsettings"
	"github.com/grafana/grafana/pkg/services/provisioning/alerting/rules"
	"github.com/grafana/grafana/pkg/services/provisioning/dashboards"
	"github.com/grafana/grafana/pkg/services/provisioning/datasources"
	"github.com/grafana/grafana/pkg/services/provisioning/notifiers"
	"github.com/grafana/grafana/pkg/services/provisioning/plugins"
	"github.com/grafana/grafana/pkg/services/provisioning/utils"
	"github.com/grafana/grafana/pkg/services/quota"
	"github.com/grafana/grafana/pkg/services/searchV2"
	"github.com/grafana/grafana/pkg/services/sqlstore"
	"github.com/grafana/grafana/pkg/setting"
)

func ProvideService(
	ac accesscontrol.AccessControl,
	cfg *setting.Cfg,
	sqlStore *sqlstore.SQLStore,
	pluginStore plugifaces.Store,
	encryptionService encryption.Internal,
	notificatonService *notifications.NotificationService,
	dashboardProvisioningService dashboardservice.DashboardProvisioningService,
	datasourceService datasourceservice.DataSourceService,
	correlationsService correlations.Service,
	dashboardService dashboardservice.DashboardService,
	folderService dashboardservice.FolderService,
	alertingService *alerting.AlertNotificationService,
	pluginSettings pluginsettings.Service,
	searchService searchV2.SearchService,
	quotaService quota.Service,
) (*ProvisioningServiceImpl, error) {
	s := &ProvisioningServiceImpl{
		Cfg:                          cfg,
		SQLStore:                     sqlStore,
		ac:                           ac,
		pluginStore:                  pluginStore,
		EncryptionService:            encryptionService,
		NotificationService:          notificatonService,
		newDashboardProvisioner:      dashboards.New,
		provisionNotifiers:           notifiers.Provision,
		provisionDatasources:         datasources.Provision,
		provisionPlugins:             plugins.Provision,
		dashboardProvisioningService: dashboardProvisioningService,
		dashboardService:             dashboardService,
		datasourceService:            datasourceService,
		correlationsService:          correlationsService,
		alertingService:              alertingService,
		pluginsSettings:              pluginSettings,
		searchService:                searchService,
		quotaService:                 quotaService,
		log:                          log.New("provisioning"),
	}
	return s, nil
}

type ProvisioningService interface {
	registry.BackgroundService
	RunInitProvisioners(ctx context.Context) error
	ProvisionDatasources(ctx context.Context) error
	ProvisionPlugins(ctx context.Context) error
	ProvisionNotifications(ctx context.Context) error
	ProvisionDashboards(ctx context.Context) error
	ProvisionAlertRules(ctx context.Context) error
	GetDashboardProvisionerResolvedPath(name string) string
	GetAllowUIUpdatesFromConfig(name string) bool
}

// Add a public constructor for overriding service to be able to instantiate OSS as fallback
func NewProvisioningServiceImpl() *ProvisioningServiceImpl {
	logger := log.New("provisioning")
	return &ProvisioningServiceImpl{
		log:                     logger,
		newDashboardProvisioner: dashboards.New,
		provisionNotifiers:      notifiers.Provision,
		provisionDatasources:    datasources.Provision,
		provisionPlugins:        plugins.Provision,
		provisionRules:          rules.Provision,
	}
}

// Used for testing purposes
func newProvisioningServiceImpl(
	newDashboardProvisioner dashboards.DashboardProvisionerFactory,
	provisionNotifiers func(context.Context, string, notifiers.Manager, notifiers.SQLStore, encryption.Internal, *notifications.NotificationService) error,
	provisionDatasources func(context.Context, string, datasources.Store, datasources.CorrelationsStore, utils.OrgStore) error,
	provisionPlugins func(context.Context, string, plugins.Store, plugifaces.Store, pluginsettings.Service) error,
) *ProvisioningServiceImpl {
	return &ProvisioningServiceImpl{
		log:                     log.New("provisioning"),
		newDashboardProvisioner: newDashboardProvisioner,
		provisionNotifiers:      provisionNotifiers,
		provisionDatasources:    provisionDatasources,
		provisionPlugins:        provisionPlugins,
	}
}

type ProvisioningServiceImpl struct {
	Cfg                          *setting.Cfg
	SQLStore                     *sqlstore.SQLStore
	ac                           accesscontrol.AccessControl
	pluginStore                  plugifaces.Store
	EncryptionService            encryption.Internal
	NotificationService          *notifications.NotificationService
	log                          log.Logger
	pollingCtxCancel             context.CancelFunc
	newDashboardProvisioner      dashboards.DashboardProvisionerFactory
	dashboardProvisioner         dashboards.DashboardProvisioner
	provisionNotifiers           func(context.Context, string, notifiers.Manager, notifiers.SQLStore, encryption.Internal, *notifications.NotificationService) error
	provisionDatasources         func(context.Context, string, datasources.Store, datasources.CorrelationsStore, utils.OrgStore) error
	provisionPlugins             func(context.Context, string, plugins.Store, plugifaces.Store, pluginsettings.Service) error
	provisionRules               func(context.Context, string, dashboardservice.DashboardService, dashboardservice.DashboardProvisioningService, provisioning.AlertRuleService) error
	mutex                        sync.Mutex
	dashboardProvisioningService dashboardservice.DashboardProvisioningService
	dashboardService             dashboardservice.DashboardService
	datasourceService            datasourceservice.DataSourceService
	correlationsService          correlations.Service
	alertingService              *alerting.AlertNotificationService
	pluginsSettings              pluginsettings.Service
	searchService                searchV2.SearchService
	quotaService                 quota.Service
}

func (ps *ProvisioningServiceImpl) RunInitProvisioners(ctx context.Context) error {
	err := ps.ProvisionDatasources(ctx)
	if err != nil {
		return err
	}

	err = ps.ProvisionPlugins(ctx)
	if err != nil {
		return err
	}

	err = ps.ProvisionNotifications(ctx)
	if err != nil {
		return err
	}

	err = ps.ProvisionAlertRules(ctx)
	if err != nil {
		return err
	}

	return nil
}

func (ps *ProvisioningServiceImpl) Run(ctx context.Context) error {
	err := ps.ProvisionDashboards(ctx)
	if err != nil {
		ps.log.Error("Failed to provision dashboard", "error", err)
		return err
	}
	if ps.dashboardProvisioner.HasDashboardSources() {
		ps.searchService.TriggerReIndex()
	}

	for {
		// Wait for unlock. This is tied to new dashboardProvisioner to be instantiated before we start polling.
		ps.mutex.Lock()
		// Using background here because otherwise if root context was canceled the select later on would
		// non-deterministically take one of the route possibly going into one polling loop before exiting.
		pollingContext, cancelFun := context.WithCancel(context.Background())
		ps.pollingCtxCancel = cancelFun
		ps.dashboardProvisioner.PollChanges(pollingContext)
		ps.mutex.Unlock()

		select {
		case <-pollingContext.Done():
			// Polling was canceled.
			continue
		case <-ctx.Done():
			// Root server context was cancelled so cancel polling and leave.
			ps.cancelPolling()
			return ctx.Err()
		}
	}
}

func (ps *ProvisioningServiceImpl) ProvisionDatasources(ctx context.Context) error {
	datasourcePath := filepath.Join(ps.Cfg.ProvisioningPath, "datasources")
	if err := ps.provisionDatasources(ctx, datasourcePath, ps.datasourceService, ps.correlationsService, ps.SQLStore); err != nil {
		err = fmt.Errorf("%v: %w", "Datasource provisioning error", err)
		ps.log.Error("Failed to provision data sources", "error", err)
		return err
	}
	return nil
}

func (ps *ProvisioningServiceImpl) ProvisionPlugins(ctx context.Context) error {
	appPath := filepath.Join(ps.Cfg.ProvisioningPath, "plugins")
	if err := ps.provisionPlugins(ctx, appPath, ps.SQLStore, ps.pluginStore, ps.pluginsSettings); err != nil {
		err = fmt.Errorf("%v: %w", "app provisioning error", err)
		ps.log.Error("Failed to provision plugins", "error", err)
		return err
	}
	return nil
}

func (ps *ProvisioningServiceImpl) ProvisionNotifications(ctx context.Context) error {
	alertNotificationsPath := filepath.Join(ps.Cfg.ProvisioningPath, "notifiers")
	if err := ps.provisionNotifiers(ctx, alertNotificationsPath, ps.alertingService, ps.SQLStore, ps.EncryptionService, ps.NotificationService); err != nil {
		err = fmt.Errorf("%v: %w", "Alert notification provisioning error", err)
		ps.log.Error("Failed to provision alert notifications", "error", err)
		return err
	}
	return nil
}

func (ps *ProvisioningServiceImpl) ProvisionDashboards(ctx context.Context) error {
	dashboardPath := filepath.Join(ps.Cfg.ProvisioningPath, "dashboards")
	dashProvisioner, err := ps.newDashboardProvisioner(ctx, dashboardPath, ps.dashboardProvisioningService, ps.SQLStore, ps.dashboardService)
	if err != nil {
		return fmt.Errorf("%v: %w", "Failed to create provisioner", err)
	}

	ps.mutex.Lock()
	defer ps.mutex.Unlock()

	ps.cancelPolling()
	dashProvisioner.CleanUpOrphanedDashboards(ctx)

	err = dashProvisioner.Provision(ctx)
	if err != nil {
		// If we fail to provision with the new provisioner, the mutex will unlock and the polling will restart with the
		// old provisioner as we did not switch them yet.
		return fmt.Errorf("%v: %w", "Failed to provision dashboards", err)
	}
	ps.dashboardProvisioner = dashProvisioner
	return nil
}

func (ps *ProvisioningServiceImpl) ProvisionAlertRules(ctx context.Context) error {
	alertRulesPath := filepath.Join(ps.Cfg.ProvisioningPath, "alerting")
	st := store.DBstore{
		Cfg:              ps.Cfg.UnifiedAlerting,
		SQLStore:         ps.SQLStore,
		Logger:           ps.log,
		FolderService:    nil, // we don't use it yet
		AccessControl:    ps.ac,
		DashboardService: ps.dashboardService,
	}
	ruleService := provisioning.NewAlertRuleService(
		st,
		st,
		ps.quotaService,
		ps.SQLStore,
		int64(ps.Cfg.UnifiedAlerting.DefaultRuleEvaluationInterval.Seconds()),
		int64(ps.Cfg.UnifiedAlerting.BaseInterval.Seconds()),
		ps.log)
	return rules.Provision(ctx, alertRulesPath, ps.dashboardService,
		ps.dashboardProvisioningService, *ruleService)
}

func (ps *ProvisioningServiceImpl) GetDashboardProvisionerResolvedPath(name string) string {
	return ps.dashboardProvisioner.GetProvisionerResolvedPath(name)
}

func (ps *ProvisioningServiceImpl) GetAllowUIUpdatesFromConfig(name string) bool {
	return ps.dashboardProvisioner.GetAllowUIUpdatesFromConfig(name)
}

func (ps *ProvisioningServiceImpl) cancelPolling() {
	if ps.pollingCtxCancel != nil {
		ps.log.Debug("Stop polling for dashboard changes")
		ps.pollingCtxCancel()
	}
	ps.pollingCtxCancel = nil
}
