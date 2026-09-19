package bqbilling

import (
	"encoding/json"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
)

func TestBillingExportRowJSONRoundTrip(t *testing.T) {
	wanted := BillingExportRow{
		BillingAccountID: "1234567890",
		Service: Service{
			ID:          bigquery.NullString{StringVal: "compute-engine", Valid: true},
			Description: bigquery.NullString{StringVal: "Compute Engine", Valid: true},
		},
		SKU: SKU{
			ID:          bigquery.NullString{StringVal: "sku-1", Valid: true},
			Description: bigquery.NullString{StringVal: "vCPU", Valid: true},
		},
		UsageStartTime: time.Date(2026, time.January, 10, 8, 0, 0, 0, time.UTC),
		UsageEndTime:   time.Date(2026, time.January, 10, 9, 0, 0, 0, time.UTC),
		Project: &Project{
			ID:              bigquery.NullString{StringVal: "project-001", Valid: true},
			Name:            bigquery.NullString{StringVal: "demo-project", Valid: true},
			Number:          bigquery.NullString{StringVal: "001", Valid: true},
			AncestryNumbers: bigquery.NullString{StringVal: "0001", Valid: true},
			Labels:          []Label{{Key: "env", Value: "dev"}},
		},
		Labels: []Label{{Key: "team", Value: "platform"}},
		Location: Location{
			Location: bigquery.NullString{StringVal: "us-central1", Valid: true},
			Country:  bigquery.NullString{StringVal: "US", Valid: true},
			Region:   bigquery.NullString{StringVal: "us-central1", Valid: true},
			Zone:     bigquery.NullString{StringVal: "us-central1-a", Valid: true},
		},
		ExportTime:             time.Date(2026, time.January, 10, 10, 0, 0, 0, time.UTC),
		Cost:                   12.5,
		Currency:               "USD",
		CurrencyConversionRate: 1.0,
		Usage: Usage{
			Amount:               3.5,
			Unit:                 bigquery.NullString{StringVal: "hour", Valid: true},
			AmountInPricingUnits: 2.25,
			PricingUnit:          bigquery.NullString{StringVal: "hour", Valid: true},
		},
		Credits: []Credit{{
			Name:     bigquery.NullString{StringVal: "commitment", Valid: true},
			Amount:   1.25,
			FullName: bigquery.NullString{StringVal: "Commitment discount", Valid: true},
			ID:       bigquery.NullString{StringVal: "credit-42", Valid: true},
			Type:     bigquery.NullString{StringVal: "sustained_use", Valid: true},
		}},
		Invoice:    Invoice{Month: "2026-01"},
		CostType:   bigquery.NullString{StringVal: "regular", Valid: true},
		CostAtList: bigquery.NullFloat64{Float64: 14.5, Valid: true},
		Resource: &Resource{
			Name:       bigquery.NullString{StringVal: "resource-1", Valid: true},
			GlobalName: bigquery.NullString{StringVal: "global-resource-1", Valid: true},
		},
		Tags: []Tag{{Key: "application", Value: "demo", Inherited: true, Namespace: "gcp"}},
		AdjustmentInfo: &Adjustment{
			ID:          bigquery.NullString{StringVal: "adj-1", Valid: true},
			Description: bigquery.NullString{StringVal: "manual adjustment", Valid: true},
			Mode:        bigquery.NullString{StringVal: "credit", Valid: true},
			Type:        bigquery.NullString{StringVal: "discount", Valid: true},
		},
	}

	data, err := json.Marshal(wanted)
	if err != nil {
		t.Fatalf("marshal BillingExportRow: %v", err)
	}

	var got BillingExportRow
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal BillingExportRow: %v", err)
	}

	if got.BillingAccountID != wanted.BillingAccountID {
		t.Fatalf("BillingAccountID mismatch: got %q want %q", got.BillingAccountID, wanted.BillingAccountID)
	}
	if got.Currency != wanted.Currency {
		t.Fatalf("Currency mismatch: got %q want %q", got.Currency, wanted.Currency)
	}
	if got.Cost != wanted.Cost {
		t.Fatalf("Cost mismatch: got %v want %v", got.Cost, wanted.Cost)
	}
	if got.Project == nil || got.Project.Name.StringVal != wanted.Project.Name.StringVal {
		t.Fatalf("Project name mismatch: got %#v want %#v", got.Project, wanted.Project)
	}
	if len(got.Credits) != len(wanted.Credits) || got.Credits[0].Name.StringVal != wanted.Credits[0].Name.StringVal {
		t.Fatalf("Credits mismatch: got %#v want %#v", got.Credits, wanted.Credits)
	}
	if got.CostType.StringVal != wanted.CostType.StringVal {
		t.Fatalf("CostType mismatch: got %q want %q", got.CostType.StringVal, wanted.CostType.StringVal)
	}
	if got.CostAtList.Float64 != wanted.CostAtList.Float64 {
		t.Fatalf("CostAtList mismatch: got %v want %v", got.CostAtList.Float64, wanted.CostAtList.Float64)
	}
}

func TestBillingExportRowNullFieldsRemainValid(t *testing.T) {
	row := BillingExportRow{
		BillingAccountID: "acct-2",
		CostType:         bigquery.NullString{Valid: false},
		CostAtList:       bigquery.NullFloat64{Valid: false},
	}

	data, err := json.Marshal(row)
	if err != nil {
		t.Fatalf("marshal null fields: %v", err)
	}

	var decoded BillingExportRow
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal null fields: %v", err)
	}

	if decoded.CostType.Valid || decoded.CostAtList.Valid {
		t.Fatalf("null fields should stay invalid after round trip: got CostType=%#v CostAtList=%#v", decoded.CostType, decoded.CostAtList)
	}
}
