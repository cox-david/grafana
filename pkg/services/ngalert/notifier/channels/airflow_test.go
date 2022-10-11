package channels

import (
	"context"
	"net/url"
	"testing"

	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"

	"github.com/grafana/grafana/pkg/components/simplejson"
	"github.com/grafana/grafana/pkg/services/secrets/fakes"
	secretsManager "github.com/grafana/grafana/pkg/services/secrets/manager"
)

func TestAirflowNotifier(t *testing.T) {
	tmpl := templateForTests(t)

	externalURL, err := url.Parse("http://localhost")
	require.NoError(t, err)
	tmpl.ExternalURL = externalURL

	cases := []struct {
		name           string
		settings       string
		alerts         []*types.Alert
		expUrl, expMsg string
		expInitError   string
		expMsgError    error
	}{
		{
			name: "A single alert",
			settings: `{
				"airflowEndpoint": "http://localhost",
				"dagID": "somedag"
			}`,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__dashboardUid__": "abcd", "__panelId__": "efgh", "__alertImageToken__": "test-image-1"},
					},
				},
			},
			expUrl: "http://localhost/api/v1/dags/somedag/dagRuns",
			expMsg: `{
				  "conf": {}
				}`,
			expMsgError: nil,
		}, {
			name: "Multiple alerts",
			settings: `{
				"airflowEndpoint": "http://localhost",
				"dagID": "somedag"
			}`,
			alerts: []*types.Alert{
				{
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val1"},
						Annotations: model.LabelSet{"ann1": "annv1", "__alertImageToken__": "test-image-1"},
					},
				}, {
					Alert: model.Alert{
						Labels:      model.LabelSet{"alertname": "alert1", "lbl1": "val2"},
						Annotations: model.LabelSet{"ann1": "annv2", "__alertImageToken__": "test-image-2"},
					},
				},
			},
			expUrl: "http://localhost/api/v1/dags/somedag/dagRuns",
			expMsg: `{
				"conf": {}
			  }`,
			expMsgError: nil,
		}, {
			name:         "Endpoint missing",
			settings:     `{"dagID": "somedag"}`,
			expInitError: `could not find Airflow API endpoint property in settings`,
		}, {
			name:         "DAG missing",
			settings:     `{"airflowEndpoint": "http://localhost"}`,
			expInitError: `could not find Airflow DAG property in settings`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			settingsJSON, err := simplejson.NewJson([]byte(c.settings))
			require.NoError(t, err)

			m := &NotificationChannelConfig{
				Name:     "airflow_testing",
				Type:     "airflow",
				Settings: settingsJSON,
			}

			webhookSender := mockNotificationService()
			secretsService := secretsManager.SetupTestService(t, fakes.NewFakeSecretsStore())
			decryptFn := secretsService.GetDecryptedValue
			cfg, err := NewAirflowConfig(m, decryptFn)
			if c.expInitError != "" {
				require.Error(t, err)
				require.Equal(t, c.expInitError, err.Error())
				return
			}
			require.NoError(t, err)

			ctx := notify.WithGroupKey(context.Background(), "alertname")
			ctx = notify.WithGroupLabels(ctx, model.LabelSet{"alertname": ""})

			pn := NewAirflowNotifier(cfg, webhookSender, tmpl)
			ok, err := pn.Notify(ctx, c.alerts...)
			if c.expMsgError != nil {
				require.False(t, ok)
				require.Error(t, err)
				require.Equal(t, c.expMsgError.Error(), err.Error())
				return
			}
			require.NoError(t, err)
			require.True(t, ok)

			require.Equal(t, c.expUrl, webhookSender.Webhook.Url)
			require.JSONEq(t, c.expMsg, webhookSender.Webhook.Body)
		})
	}
}
