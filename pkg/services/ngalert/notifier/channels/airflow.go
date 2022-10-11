package channels

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/infra/log"
	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/services/notifications"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
)

const (
	airflowUrl string = "%s/api/v1/dags/%s/dagRuns"
)

type AirflowConfig struct {
	*NotificationChannelConfig
	URL         string
	DagID       string
	User        string
	Password    string
	RunId       string
	LogicalDate string
	State       string
	Conf        string
}

func AirflowFactory(fc FactoryConfig) (NotificationChannel, error) {
	cfg, err := NewAirflowConfig(fc.Config, fc.DecryptFunc)
	if err != nil {
		return nil, receiverInitError{
			Reason: err.Error(),
			Cfg:    *fc.Config,
		}
	}
	return NewAirflowNotifier(cfg, fc.NotificationService, fc.Template), nil
}

func NewAirflowConfig(config *NotificationChannelConfig, decryptFunc GetDecryptedValueFn) (*AirflowConfig, error) {
	dagId := config.Settings.Get("dagId").MustString()
	if dagId == "" {
		return nil, errors.New("Could not find DAG ID property in settings")
	}

	url := config.Settings.Get("url").MustString()
	if url == "" {
		return nil, errors.New("Could not find url property in settings")
	}

	password := decryptFunc(context.Background(), config.SecureSettings, "password", config.Settings.Get("password").MustString())
	userName := config.Settings.Get("username").MustString()
	runId := config.Settings.Get("runId").MustString()
	logicalDate := config.Settings.Get("logicalDate").MustString()
	state := config.Settings.Get("state").MustString()
	conf := strings.ReplaceAll(config.Settings.Get("conf").MustString(), "'", "\"")

	return &AirflowConfig{
		NotificationChannelConfig: config,
		URL:                       url,
		DagID:                     dagId,
		User:                      userName,
		Password:                  password,
		RunId:                     runId,
		LogicalDate:               logicalDate,
		State:                     state,
		Conf:                      conf,
	}, nil
}

// NewAirflowNotifier is the constructor for the Airflow notifier
func NewAirflowNotifier(config *AirflowConfig, ns notifications.WebhookSender, t *template.Template) *AirflowNotifier {
	return &AirflowNotifier{
		Base: NewBase(&models.AlertNotification{
			Uid:                   config.UID,
			Name:                  config.Name,
			Type:                  config.Type,
			DisableResolveMessage: config.DisableResolveMessage,
			Settings:              config.Settings,
		}),
		URL:         config.URL,
		DagID:       config.DagID,
		User:        config.User,
		Password:    config.Password,
		RunId:       config.RunId,
		LogicalDate: config.LogicalDate,
		State:       config.State,
		Conf:        config.Conf,
		log:         log.New("alerting.notifier.airflow"),
		ns:          ns,
		tmpl:        t,
	}
}

// AirflowNotifier is responsible for sending
// alert notifications to Airflow.
type AirflowNotifier struct {
	*Base
	URL         string
	DagID       string
	User        string
	Password    string
	RunId       string
	LogicalDate string
	State       string
	Conf        string
	log         log.Logger
	ns          notifications.WebhookSender
	tmpl        *template.Template
}

// Notify send an alert notification to Airflow
func (an *AirflowNotifier) Notify(ctx context.Context, as ...*types.Alert) (bool, error) {
	an.log.Debug("executing airflow notification", "notification", an.Name)

	var conf map[string]interface{}
	if err := json.Unmarshal([]byte(an.Conf), &conf); err != nil {
		conf = make(map[string]interface{})
	}

	var tmplErr error
	tmpl, _ := TmplText(ctx, an.tmpl, as, an.log, &tmplErr)

	content := map[string]string{
		"client":      "Grafana",
		"client_url":  joinUrlPath(an.tmpl.ExternalURL.String(), "/alerting/list", an.log),
		"description": tmpl(DefaultMessageTitleEmbed),
		"details":     tmpl(DefaultMessageEmbed),
	}

	conf["trigger"] = content

	bodyJSON := simplejson.New()

	if an.RunId != "" {
		bodyJSON.Set("dag_run_id", an.RunId)
	}
	if an.LogicalDate != "" {
		bodyJSON.Set("logical_date", an.LogicalDate)
	}
	if an.State != "" {
		bodyJSON.Set("state", an.State)
	}
	bodyJSON.Set("conf", conf)

	body, err := bodyJSON.MarshalJSON()
	if err != nil {
		return false, err
	}

	endPoint := fmt.Sprintf(airflowUrl, an.URL, an.DagID)

	cmd := &models.SendWebhookSync{
		Url:        endPoint,
		User:       an.User,
		Password:   an.Password,
		HttpMethod: http.MethodPost,
		HttpHeader: map[string]string{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		Body: string(body),
	}

	if err := an.ns.SendWebhookSync(ctx, cmd); err != nil {
		an.log.Error("failed to send notification to Airflow", "err", err, "body", string(body))
		return false, err
	}

	return true, nil
}

func (an *AirflowNotifier) SendResolved() bool {
	return !an.GetDisableResolveMessage()
}
