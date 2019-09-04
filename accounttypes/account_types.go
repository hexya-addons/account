// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package accounttypes

import (
	"github.com/hexya-erp/hexya/src/models/types/dates"
)

// A PaymentDueDates gives the amount due of an invoice at the given date
type PaymentDueDates struct {
	Date   dates.Date
	Amount float64
}

// A TaxGroup holds an amount for a given group name
type TaxGroup struct {
	GroupName string
	TaxAmount float64
	Sequence  int
}

// A DataForReconciliationWidget holds data for the reconciliation widget
type DataForReconciliationWidget struct {
	Customers []map[string]interface{} `json:"customers"`
	Suppliers []map[string]interface{} `json:"suppliers"`
	Accounts  []map[string]interface{} `json:"accounts"`
}

// An AppliedTaxData is the result of the computation of applying a tax on an amount.
type AppliedTaxData struct {
	ID              int64   `json:"id"`
	Name            string  `json:"name"`
	Amount          float64 `json:"amount"`
	Sequence        int     `json:"sequence"`
	AccountID       int64   `json:"account_id"`
	RefundAccountID int64   `json:"refund_account_id"`
	Analytic        bool    `json:"analytic"`
	Base            float64 `json:"base"`
}

type TransRecGetStruct struct {
	TransNbr int64
	Credit   float64
	Debit    float64
	WriteOff float64
}

// BankStatementAMLStruct is a temporary struct for holding AccountMoveLine
// data during bank statement import
type BankStatementAMLStruct struct {
	Name             string
	Debit            float64
	Credit           float64
	AmountCurrency   float64
	MoveLineID       int64
	AccountID        int64
	CurrencyID       int64
	MoveID           int64
	PartnerID        int64
	StatementID      int64
	PaymentID        int64
	CounterpartAMLID int64
	JournalID        int64
}

// InvoiceLineAMLStruct is a temporary struct for holding AccountMoveLine
// data during invoice validation.
type InvoiceLineAMLStruct struct {
	InvoiceLineID     int64
	InvoiceLineTaxID  int64
	TaxLineID         int64
	Type              string
	Name              string
	PriceUnit         float64
	Quantity          float64
	Price             float64
	AmountCurrency    float64
	AccountID         int64
	ProductID         int64
	UomID             int64
	AccountAnalyticID int64
	CurrencyID        int64
	TaxIDs            []int64
	InvoiceID         int64
	AnalyticTagsIDs   []int64
	AnalyticLinesIDs  []int64
	DateMaturity      dates.Date
}

// AgedBalanceReportValues holds data to render the aged partner balance report
type AgedBalanceReportValues struct {
	Direction float64
	Values    [5]float64
	Total     float64
	PartnerID int64
	Name      string
	Trust     bool
}

// AgedBalanceReportLine holds data to render the aged partner balance report
type AgedBalanceReportLine struct {
	LineID int64
	Amount float64
	Period int
}

// AgedBalancePeriod holds data of a period in the aged partner balance report
type AgedBalancePeriod struct {
	Name  string
	Start dates.Date
	Stop  dates.Date
}
