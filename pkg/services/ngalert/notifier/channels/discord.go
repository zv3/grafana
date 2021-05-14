package channels

import (
	"context"
	"encoding/json"

	"github.com/pkg/errors"

	"github.com/grafana/grafana/pkg/bus"
	"github.com/grafana/grafana/pkg/setting"

	"github.com/grafana/grafana/pkg/components/simplejson"

	gokit_log "github.com/go-kit/kit/log"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/alerting"
	old_notifiers "github.com/grafana/grafana/pkg/services/alerting/notifiers"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
)

type DiscordNotifier struct {
	old_notifiers.NotifierBase
	log         log.Logger
	tmpl        *template.Template
	Description string
	Message     string
	WebhookURL  string
}

func NewDiscordNotifier(model *models.AlertNotification, t *template.Template) (*DiscordNotifier, error) {
	if model.Settings == nil {
		return nil, alerting.ValidationError{Reason: "No Settings Supplied"}
	}

	discordURL := model.Settings.Get("url").MustString()
	if discordURL == "" {
		return nil, alerting.ValidationError{Reason: "Could not find webhook url property in settings"}
	}

	message := model.Settings.Get("message").MustString()

	return &DiscordNotifier{
		NotifierBase: old_notifiers.NewNotifierBase(model),
		Message:      message,
		WebhookURL:   discordURL,
		log:          log.New("alerting.notifier.discord"),
		tmpl:         t,
	}, nil
}

func (d DiscordNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	data := notify.GetTemplateData(ctx, d.tmpl, as, gokit_log.NewNopLogger())
	alerts := types.Alerts(as...)

	bodyJSON := simplejson.New()
	bodyJSON.Set("username", "Grafana")

	var tmplErr error
	tmpl := notify.TmplText(d.tmpl, data, &tmplErr)
	if d.Message != "" {
		bodyJSON.Set("content", tmpl(d.Message))
	}
	if tmplErr != nil {
		return false, errors.Wrap(tmplErr, "failed to template discord message")
	}

	fields := make([]map[string]interface{}, 0)
	for _, a := range as {
		fields = append(fields, map[string]interface{}{
			"name":   a.Labels.String(),
			"inline": true,
		})
	}

	footer := map[string]interface{}{
		"text":     "Grafana v" + setting.BuildVersion,
		"icon_url": "https://grafana.com/assets/img/fav32.png",
	}

	embed := simplejson.New()
	embed.Set("title", getTitleFromTemplateData(data))
	embed.Set("color", getAlertStatusColor(alerts.Status()))
	embed.Set("footer", footer)
	embed.Set("fields", fields)

	if d.Description != "" {
		embed.Set("description", d.Description)
	}

	ruleURL := d.tmpl.ExternalURL.String() + "/alerting/list"
	embed.Set("url", ruleURL)

	bodyJSON.Set("embeds", []interface{}{embed})
	body, err := json.Marshal(bodyJSON)
	if err != nil {
		return false, err
	}
	cmd := &models.SendWebhookSync{
		Url:         d.WebhookURL,
		HttpMethod:  "POST",
		ContentType: "application/json",
		Body:        string(body),
	}

	if err := bus.DispatchCtx(ctx, cmd); err != nil {
		d.log.Error("Failed to send notification to Discord", "error", err)
		return false, err
	}
	return true, nil
}

func (d DiscordNotifier) SendResolved() bool {
	return !d.GetDisableResolveMessage()
}
