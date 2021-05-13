package channels

import (
	"fmt"
	"strings"

	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/common/model"

	"github.com/grafana/grafana/pkg/components/securejsondata"
	"github.com/grafana/grafana/pkg/components/simplejson"
)

const (
	FooterIconURL      = "https://grafana.com/assets/img/fav32.png"
	ColorAlertFiring   = "#D63232"
	ColorAlertResolved = "#36a64f"
)

func getAlertStatusColor(status model.AlertStatus) string {
	if status == model.AlertFiring {
		return ColorAlertFiring
	}
	return ColorAlertResolved
}

func getTitleFromTemplateData(data *template.Data) string {
	title := "[" + data.Status
	if data.Status == string(model.AlertFiring) {
		title += fmt.Sprintf(":%d", len(data.Alerts.Firing()))
	}
	title += "] " + strings.Join(data.GroupLabels.SortedPairs().Values(), " ") + " "
	if len(data.CommonLabels) > len(data.GroupLabels) {
		title += "(" + strings.Join(data.CommonLabels.Remove(data.GroupLabels.Names()).Values(), " ") + ")"
	}
	return title
}

type NotificationChannelConfig struct {
	Name                  string                        `json:"name"`
	Type                  string                        `json:"type"`
	DisableResolveMessage bool                          `json:"disableResolveMessage"`
	Settings              *simplejson.Json              `json:"settings"`
	SecureSettings        securejsondata.SecureJsonData `json:"secureSettings"`
}

// DecryptedValue returns decrypted value from secureSettings
func (an *NotificationChannelConfig) DecryptedValue(field string, fallback string) string {
	if value, ok := an.SecureSettings.DecryptedValue(field); ok {
		return value
	}
	return fallback
}
