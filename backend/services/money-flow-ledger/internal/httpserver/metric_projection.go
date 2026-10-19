package httpserver

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"omsbizfoundry/money-flow-ledger/internal/identity"
	"omsbizfoundry/money-flow-ledger/internal/operatingsummary"
)

const operatingMetricDefinitionVersion = "money-flow-operating-summary-v1"

type metricProjection struct {
	SchemaVersion     string                 `json:"schema_version"`
	SourceApplication string                 `json:"source_application"`
	TenantID          string                 `json:"tenant_id"`
	DefinitionVersion string                 `json:"definition_version"`
	ObservedAt        string                 `json:"observed_at"`
	Items             []metricProjectionItem `json:"items"`
}

type metricProjectionItem struct {
	MetricKey    string                `json:"metric_key"`
	Scope        metricProjectionScope `json:"scope"`
	Unit         string                `json:"unit"`
	Currency     string                `json:"currency"`
	Value        *string               `json:"value"`
	Redacted     bool                  `json:"redacted"`
	SourceDigest *string               `json:"source_digest"`
	TimeRange    string                `json:"time_range"`
	ObservedAt   string                `json:"observed_at"`
	DeepLink     string                `json:"deep_link"`
}

type metricProjectionScope struct {
	Kind         string `json:"kind"`
	TenantID     string `json:"tenant_id"`
	ResourceType string `json:"resource_type"`
	ResourceID   string `json:"resource_id"`
}

func (s *Server) operatingMetricProjection(w http.ResponseWriter, r *http.Request) {
	principal, ok := principalFromContext(r.Context())
	if !ok || principal.TenantID == "" {
		http.Error(w, "无法确定当前租户", http.StatusInternalServerError)
		return
	}
	report, err := s.operatingSummaries.Current(r.Context())
	if err != nil {
		http.Error(w, "经营指标来源暂时不可用", http.StatusServiceUnavailable)
		return
	}
	projection := buildOperatingMetricProjection(principal, report)
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if err := json.NewEncoder(w).Encode(projection); err != nil {
		http.Error(w, "无法编码经营指标", http.StatusInternalServerError)
	}
}

func buildOperatingMetricProjection(principal identity.Principal, report operatingsummary.Report) metricProjection {
	observedAt := report.GeneratedAt.UTC().Format(time.RFC3339Nano)
	result := metricProjection{
		SchemaVersion: "1", SourceApplication: serviceName, TenantID: principal.TenantID,
		DefinitionVersion: operatingMetricDefinitionVersion, ObservedAt: observedAt,
		Items: make([]metricProjectionItem, 0, len(report.Groups)*6),
	}
	for _, group := range report.Groups {
		metrics := []operatingsummary.AmountMetric{
			group.ReceivableOutstanding, group.PayableOutstanding, group.ReceiptEffective,
			group.PaymentEffective, group.ReceiptUnallocated, group.PaymentUnallocated,
		}
		for _, metric := range metrics {
			item := metricProjectionItem{
				MetricKey: string(metric.Metric),
				Scope: metricProjectionScope{
					Kind: "ledger_owner", TenantID: principal.TenantID,
					ResourceType: "party", ResourceID: group.LedgerOwnerPartyID,
				},
				Unit: "money", Currency: group.Currency, Redacted: true,
				TimeRange: "current", ObservedAt: observedAt,
				DeepLink: "/operating-summary/details?metric=" + url.QueryEscape(string(metric.Metric)) + "&ledger_owner_party_id=" + url.QueryEscape(group.LedgerOwnerPartyID) + "&currency=" + url.QueryEscape(group.Currency),
			}
			if principal.Can(identity.PermissionAmountRead) {
				value := metric.Amount
				digest := operatingMetricDigest(principal.TenantID, group.LedgerOwnerPartyID, group.Currency, metric)
				item.Value = &value
				item.SourceDigest = &digest
				item.Redacted = false
			}
			result.Items = append(result.Items, item)
		}
	}
	return result
}

func operatingMetricDigest(tenantID, ownerID, currency string, metric operatingsummary.AmountMetric) string {
	value := operatingMetricDefinitionVersion + "\x00" + tenantID + "\x00" + ownerID + "\x00" + currency + "\x00" + string(metric.Metric) + "\x00" + metric.Amount + "\x00" + strconv.FormatInt(metric.FactCount, 10)
	sum := sha256.Sum256([]byte(value))
	return "sha256:" + hex.EncodeToString(sum[:])
}
