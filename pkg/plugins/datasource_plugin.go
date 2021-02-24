package plugins

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins/backendplugin"
	"github.com/grafana/grafana/pkg/plugins/backendplugin/grpcplugin"
	"github.com/grafana/grafana/pkg/plugins/datasource/wrapper"
	"github.com/grafana/grafana/pkg/tsdb"
	"github.com/grafana/grafana/pkg/util/errutil"
	"gopkg.in/macaron.v1"
)

// DataSourcePlugin contains all metadata about a datasource plugin
type DataSourcePlugin struct {
	FrontendPluginBase
	Annotations  bool              `json:"annotations"`
	Metrics      bool              `json:"metrics"`
	Alerting     bool              `json:"alerting"`
	Explore      bool              `json:"explore"`
	Table        bool              `json:"tables"`
	Logs         bool              `json:"logs"`
	Tracing      bool              `json:"tracing"`
	QueryOptions map[string]bool   `json:"queryOptions,omitempty"`
	BuiltIn      bool              `json:"builtIn,omitempty"`
	Mixed        bool              `json:"mixed,omitempty"`
	Routes       []*AppPluginRoute `json:"routes"`
	Streaming    bool              `json:"streaming"`

	Backend    bool   `json:"backend,omitempty"`
	Executable string `json:"executable,omitempty"`
	SDK        bool   `json:"sdk,omitempty"`
}

func (p *DataSourcePlugin) Load(decoder *json.Decoder, base *PluginBase, backendPluginManager backendplugin.Manager) error {
	if err := decoder.Decode(p); err != nil {
		return errutil.Wrapf(err, "Failed to decode datasource plugin")
	}

	if err := p.registerPlugin(base); err != nil {
		return errutil.Wrapf(err, "Failed to register plugin")
	}

	if p.Backend {
		cmd := ComposePluginStartCommand(p.Executable)
		fullpath := filepath.Join(p.PluginDir, cmd)
		factory := grpcplugin.NewUnmanagedBackendPlugin(p.Id, fullpath, grpcplugin.PluginStartFuncs{
			OnLegacyStart: p.onLegacyPluginStart,
			OnStart:       p.onPluginStart,
		})
		if err := backendPluginManager.Register(p.Id, factory); err != nil {
			return errutil.Wrapf(err, "Failed to register backend plugin")
		}
	}

	DataSources[p.Id] = p
	return nil
}

func (p *DataSourcePlugin) onLegacyPluginStart(pluginID string, client *grpcplugin.LegacyClient, logger log.Logger) error {
	tsdb.RegisterTsdbQueryEndpoint(pluginID, func(dsInfo *models.DataSource) (tsdb.TsdbQueryEndpoint, error) {
		return wrapper.NewDatasourcePluginWrapper(logger, client.DatasourcePlugin), nil
	})

	return nil
}

func (p *DataSourcePlugin) onPluginStart(pluginID string, client *grpcplugin.Client, logger log.Logger) error {
	if client.DataPlugin != nil {
		tsdb.RegisterTsdbQueryEndpoint(pluginID, func(dsInfo *models.DataSource) (tsdb.TsdbQueryEndpoint, error) {
			return wrapper.NewDatasourcePluginWrapperV2(logger, p.Id, p.Type, client.DataPlugin), nil
		})
	}

	return nil
}

func (p *DataSourcePlugin) initDatasourcePlugin(m *macaron.Macaron, backendPluginManager backendplugin.Manager) error {
	if p.Initialized {
		return nil
	}
	p.initFrontendPlugin(m)

	if p.Backend {
		err := backendPluginManager.StartPlugin(context.Background(), p.Id)
		if err != nil {
			return fmt.Errorf("problem encountered starting backend plugin %s : %w", p.Id, err)
		}
	}
	return nil
}
