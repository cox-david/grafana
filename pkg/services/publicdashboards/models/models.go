package models

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/tsdb/legacydata"
)

// PublicDashboardErr represents a dashboard error.
type PublicDashboardErr struct {
	StatusCode int
	Status     string
	Reason     string
}

// Error returns the error message.
func (e PublicDashboardErr) Error() string {
	if e.Reason != "" {
		return e.Reason
	}
	return "Dashboard Error"
}

const QuerySuccess = "success"
const QueryFailure = "failure"

var QueryResultStatuses = []string{QuerySuccess, QueryFailure}

var (
	ErrPublicDashboardFailedGenerateUniqueUid = PublicDashboardErr{
		Reason:     "failed to generate unique public dashboard id",
		StatusCode: 500,
	}
	ErrPublicDashboardFailedGenerateAccesstoken = PublicDashboardErr{
		Reason:     "failed to public dashboard access token",
		StatusCode: 500,
	}
	ErrPublicDashboardNotFound = PublicDashboardErr{
		Reason:     "public dashboard not found",
		StatusCode: 404,
		Status:     "not-found",
	}
	ErrPublicDashboardPanelNotFound = PublicDashboardErr{
		Reason:     "panel not found in dashboard",
		StatusCode: 404,
		Status:     "not-found",
	}
	ErrPublicDashboardIdentifierNotSet = PublicDashboardErr{
		Reason:     "no Uid for public dashboard specified",
		StatusCode: 400,
	}
	ErrPublicDashboardHasTemplateVariables = PublicDashboardErr{
		Reason:     "public dashboard has template variables",
		StatusCode: 422,
	}
	ErrPublicDashboardBadRequest = PublicDashboardErr{
		Reason:     "bad Request",
		StatusCode: 400,
	}
)

type PublicDashboard struct {
	Uid          string        `json:"uid" xorm:"pk uid"`
	DashboardUid string        `json:"dashboardUid" xorm:"dashboard_uid"`
	OrgId        int64         `json:"-" xorm:"org_id"` // Don't ever marshal orgId to Json
	TimeSettings *TimeSettings `json:"timeSettings" xorm:"time_settings"`
	IsEnabled    bool          `json:"isEnabled" xorm:"is_enabled"`
	AccessToken  string        `json:"accessToken" xorm:"access_token"`

	CreatedBy int64 `json:"createdBy" xorm:"created_by"`
	UpdatedBy int64 `json:"updatedBy" xorm:"updated_by"`

	CreatedAt time.Time `json:"createdAt" xorm:"created_at"`
	UpdatedAt time.Time `json:"updatedAt" xorm:"updated_at"`
}

func (pd PublicDashboard) TableName() string {
	return "dashboard_public"
}

type TimeSettings struct {
	From string `json:"from,omitempty"`
	To   string `json:"to,omitempty"`
}

func (ts *TimeSettings) FromDB(data []byte) error {
	return json.Unmarshal(data, ts)
}

func (ts *TimeSettings) ToDB() ([]byte, error) {
	return json.Marshal(ts)
}

// build time settings object from json on public dashboard. If empty, use
// defaults on the dashboard
func (pd PublicDashboard) BuildTimeSettings(dashboard *models.Dashboard) TimeSettings {
	from := dashboard.Data.GetPath("time", "from").MustString()
	to := dashboard.Data.GetPath("time", "to").MustString()
	timeRange := legacydata.NewDataTimeRange(from, to)

	// Were using epoch ms because this is used to build a MetricRequest, which is used by query caching, which expected the time range in epoch milliseconds.
	ts := TimeSettings{
		From: strconv.FormatInt(timeRange.GetFromAsMsEpoch(), 10),
		To:   strconv.FormatInt(timeRange.GetToAsMsEpoch(), 10),
	}

	if pd.TimeSettings == nil {
		return ts
	}

	return ts
}

// DTO for transforming user input in the api
type SavePublicDashboardConfigDTO struct {
	DashboardUid    string
	OrgId           int64
	UserId          int64
	PublicDashboard *PublicDashboard
}

type PublicDashboardQueryDTO struct {
	IntervalMs    int64
	MaxDataPoints int64
}

//
// COMMANDS
//

type SavePublicDashboardConfigCommand struct {
	PublicDashboard PublicDashboard
}
