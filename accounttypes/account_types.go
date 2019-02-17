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

// mapping invoice type to refund type
var TYPE2REFUND = map[string]string{
	`out_invoice`: `out_refund`,  // Customer Invoice
	`in_invoice`:  `in_refund`,   // Vendor Bill
	`out_refund`:  `out_invoice`, // Customer Refund
	`in_refund`:   `in_invoice`,  // Vendor Refund
}

var MapInvoiceType_PartnerType = map[string]string{
	`out_invoice`: `customer`,
	`out_refund`:  `customer`,
	`in_invoice`:  `supplier`,
	`in_refund`:   `supplier`,
}

// Since invoice amounts are unsigned, this is how we know if money comes in or goes out
var MapInvoiceType_PaymentSign = map[string]float64{
	`out_invoice`: 1.0,
	`in_refund`:   1.0,
	`in_invoice`:  -1.0,
	`out_refund`:  -1.0,
}
