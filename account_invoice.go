// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"encoding/json"
	"fmt"
	"math"

	"github.com/hexya-addons/account/accounttypes"
	"github.com/hexya-addons/decimalPrecision"
	"github.com/hexya-addons/web/webdata"
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
	"github.com/hexya-erp/hexya/src/tools/strutils"
	"github.com/hexya-erp/hexya/src/views"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

var ReferenceType = types.Selection{
	"none": "Free Reference",
}

var Type2Journal = map[string]string{
	"out_invoice": "sale",
	"in_invoice":  "purchase",
	"out_refund":  "sale",
	"in_refund":   "purchase",
}

func init() {

	h.AccountInvoice().DeclareModel()
	h.AccountInvoice().SetDefaultOrder("DateInvoice DESC", "Number DESC", "ID DESC")

	h.AccountInvoice().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String: "Reference/Description",
			Index:  true, /*[ readonly True]*/ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/
			NoCopy: true,
			Help:   "The name that will be used on account move lines"},
		"Origin": models.CharField{
			String: "Source Document",
			Help:   "Reference of the document that produced this invoice." /*[ readonly True]*/ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/},
		"Type": models.SelectionField{
			Selection: types.Selection{
				"out_invoice": "Customer Invoice",
				"in_invoice":  "Vendor Bill",
				"out_refund":  "Customer Refund",
				"in_refund":   "Vendor Refund"},
			ReadOnly: true,
			Index:    true,
			Default: func(env models.Environment) interface{} {
				if env.Context().HasKey("type") {
					return env.Context().GetString("type")
				}
				return "out_invoice"
			} /*[ track_visibility 'always']*/},
		"RefundInvoice": models.Many2OneField{
			String:        "Invoice for which this invoice is the refund",
			RelationModel: h.AccountInvoice()},
		"Number": models.CharField{
			Related:  "Move.Name",
			ReadOnly: true,
			NoCopy:   true},
		"MoveName": models.CharField{
			String:  "Journal Entry Name",
			NoCopy:  true,
			Default: models.DefaultValue(false),
			Help: `Technical field holding the number given to the invoice, automatically set when the invoice is
validated then stored to set the same number again if the invoice is cancelled,
set to draft and re-validated.`},
		"Reference": models.CharField{
			String: "Vendor Reference",
			Help:   "The partner reference of this invoice." /*[ readonly True]*/ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/},
		"ReferenceType": models.SelectionField{
			Selection: ReferenceType,
			String:    "Payment Reference", /* states draft:"readonly": "False" */
			Default:   models.DefaultValue("none"),
			Required:  true},
		"Comment": models.TextField{
			String: "Additional Information" /*[ readonly True]*/ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/},
		"State": models.SelectionField{
			String: "Status",
			Selection: types.Selection{
				"draft":     "Draft",
				"proforma":  "Pro-forma",
				"proforma2": "Pro-forma",
				"open":      "Open",
				"paid":      "Paid",
				"cancel":    "Cancelled"},
			Index:    true,
			ReadOnly: true,
			Default:  models.DefaultValue("draft"), /*[ track_visibility 'onchange']*/
			NoCopy:   true,
			Help: ` * The 'Draft' status is used when a user is encoding a new and unconfirmed Invoice.
 * The 'Pro-forma' status is used when the invoice does not have an invoice number.
 * The 'Open' status is used when user creates invoice an invoice number is generated.
   It stays in the open status till the user pays the invoice.
 * The 'Paid' status is set automatically when the invoice is paid. Its related journal
   entries may or may not be reconciled.
 * The 'Cancelled' status is used when user cancel invoice.`},
		"Sent": models.BooleanField{
			ReadOnly: true,
			Default:  models.DefaultValue(false),
			NoCopy:   true,
			Help:     "It indicates that the invoice has been sent."},
		"DateInvoice": models.DateField{String: "Invoice Date", /*[ readonly True]*/ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/
			Index: true, Help: "Keep empty to use the current date", NoCopy: true,
			Constraint: h.AccountInvoice().Methods().OnchangePaymentTermDateInvoice()},
		"DateDue": models.DateField{String: "Due Date", /*[ readonly True]*/ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/
			Index: true, NoCopy: true,
			Help: `If you use payment terms, the due date will be computed automatically at the generation
of accounting entries. The payment term may compute several due dates, for example 50%
now and 50% in one month, but if you want to force a due date, make sure that the payment
term is not set on the invoice. If you keep the payment term and the due date empty, it
means direct payment.`},
		"Partner": models.Many2OneField{
			RelationModel: h.Partner(),
			Required:      true, /* readonly=true */ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/ /*[ track_visibility 'always']*/
			OnChange:      h.AccountInvoice().Methods().OnchangePartner()},
		"PaymentTerm": models.Many2OneField{
			String:        "Payment Terms",
			RelationModel: h.AccountPaymentTerm(), /* readonly=true */ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/
			Constraint:    h.AccountInvoice().Methods().OnchangePaymentTermDateInvoice(),
			Help: `If you use payment terms, the due date will be computed automatically at the generation
of accounting entries. If you keep the payment term and the due date empty, it means direct payment.
The payment term may compute several due dates, for example 50% now, 50% in one month.`},
		"Date": models.DateField{
			String: "Accounting Date",
			NoCopy: true,
			Help:   "Keep empty to use the invoice date." /*[ readonly True]*/ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/},
		"Account": models.Many2OneField{
			String:        "Account",
			RelationModel: h.AccountAccount(),
			Required:      true, /* readonly=true */ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Help:          "The partner account used for this invoice."},
		"InvoiceLines": models.One2ManyField{
			String:        "Invoice Lines",
			RelationModel: h.AccountInvoiceLine(),
			ReverseFK:     "Invoice",
			JSON:          "invoice_line_ids", /* readonly */ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/
			OnChange:      h.AccountInvoice().Methods().OnchangeInvoiceLines(),
			Copy:          true},
		"TaxLines": models.One2ManyField{
			RelationModel: h.AccountInvoiceTax(),
			ReverseFK:     "Invoice",
			JSON:          "tax_line_ids", /* readonly */ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/
			Copy:          true},
		"Move": models.Many2OneField{
			String:        "Journal Entry",
			RelationModel: h.AccountMove(),
			ReadOnly:      true,
			Index:         true,
			OnDelete:      models.Restrict,
			NoCopy:        true,
			Help:          "Link to the automatically generated Journal Items."},
		"AmountUntaxed": models.FloatField{
			String:  "Untaxed Amount",
			Stored:  true,
			Compute: h.AccountInvoice().Methods().ComputeAmount(), /*[ track_visibility 'always']*/
			Depends: []string{"InvoiceLines.PriceSubtotal", "TaxLines.Amount", "Currency", "Company", "DateInvoice", "Type"}},
		"AmountUntaxedSigned": models.FloatField{
			String:  "Untaxed Amount in Company Currency",
			Stored:  true,
			Compute: h.AccountInvoice().Methods().ComputeAmount(),
			Depends: []string{"InvoiceLines.PriceSubtotal", "TaxLines.Amount", "Currency", "Company", "DateInvoice", "Type"}},
		"AmountTax": models.FloatField{
			String:  "Tax",
			Stored:  true,
			Compute: h.AccountInvoice().Methods().ComputeAmount(),
			Depends: []string{"InvoiceLines.PriceSubtotal", "TaxLines.Amount", "Currency", "Company", "DateInvoice", "Type"}},
		"AmountTotal": models.FloatField{
			String:  "Total",
			Stored:  true,
			Compute: h.AccountInvoice().Methods().ComputeAmount(),
			Depends: []string{"InvoiceLines.PriceSubtotal", "TaxLines.Amount", "Currency", "Company", "DateInvoice", "Type"}},
		"AmountTotalSigned": models.FloatField{
			String:  "Total in Invoice Currency",
			Stored:  true,
			Compute: h.AccountInvoice().Methods().ComputeAmount(),
			Depends: []string{"InvoiceLines.PriceSubtotal", "TaxLines.Amount", "Currency", "Company", "DateInvoice", "Type"},
			Help:    "Total amount in the currency of the invoice, negative for credit notes."},
		"AmountTotalCompanySigned": models.FloatField{
			String:  "Total in Company Currency",
			Stored:  true,
			Compute: h.AccountInvoice().Methods().ComputeAmount(),
			Depends: []string{"InvoiceLines.PriceSubtotal", "TaxLines.Amount", "Currency", "Company", "DateInvoice", "Type"},
			Help:    "Total amount in the currency of the company, negative for credit notes."},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency(),
			Required:      true, /* readonly=true */ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/
			Default: func(env models.Environment) interface{} {
				journal := h.AccountInvoice().NewSet(env).DefaultJournal()
				return h.Currency().Coalesce(journal.Currency(), journal.Company().Currency(), h.User().NewSet(env).CurrentUser().Company().Currency())
			} /*[ track_visibility 'always']*/},
		"CompanyCurrency": models.Many2OneField{
			RelationModel: h.Currency(),
			Related:       "Company.Currency",
			ReadOnly:      true},
		"Journal": models.Many2OneField{
			RelationModel: h.AccountJournal(),
			Required:      true, /* readonly=true */ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/
			OnChange:      h.AccountInvoice().Methods().OnchangeJournal(),
			Default: func(env models.Environment) interface{} {
				return h.AccountInvoice().NewSet(env).DefaultJournal()
			} /*Filter: "[('type'*/ /*[ 'in']*/ /*[ {'out_invoice': ['sale']]*/ /*[ 'out_refund': ['sale']]*/ /*[ 'in_refund': ['purchase']] [ 'in_invoice': ['purchase']}.get(type,  ('company_id']*/ /*[ ' ']*/ /*[ company_id)]"]*/},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Required:      true, /* readonly=true */ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/
			Default: func(env models.Environment) interface{} {
				return h.Company().NewSet(env).CompanyDefaultGet()
			},
			OnChange: h.AccountInvoice().Methods().OnchangePartner()},
		"Reconciled": models.BooleanField{
			String:  "Paid/Reconciled",
			Stored:  true,
			Compute: h.AccountInvoice().Methods().ComputeResidual(),
			Depends: []string{"State", "Currency", "InvoiceLines.PriceSubtotal", "Move.Lines.AmountResidual", "Move.Lines.Currency"},
			Help: `It indicates that the invoice has been paid and the journal entry of the invoice
has been reconciled with one or several journal entries of payment.`},
		"PartnerBank": models.Many2OneField{
			String:        "Bank Account",
			RelationModel: h.BankAccount(),
			Help: `Bank Account Number to which the invoice will be paid.
A Company bank account if this is a Customer Invoice or Vendor Refund, otherwise a Partner bank account number.`, /* readonly=True */ /* states={'draft': [('readonly', False)]}" */
		},
		"Residual": models.FloatField{
			String:  "Amount Due",
			Compute: h.AccountInvoice().Methods().ComputeResidual(),
			Stored:  true,
			Help:    "Remaining amount due.",
			Depends: []string{"State", "Currency", "InvoiceLines.PriceSubtotal", "Move.Lines.AmountResidual", "Move.Lines.Currency"}},
		"ResidualSigned": models.FloatField{
			String:  "Amount Due in Invoice Currency",
			Compute: h.AccountInvoice().Methods().ComputeResidual(),
			Stored:  true,
			Depends: []string{"State", "Currency", "InvoiceLines.PriceSubtotal", "Move.Lines.AmountResidual", "Move.Lines.Currency"},
			Help:    "Remaining amount due in the currency of the invoice."},
		"ResidualCompanySigned": models.FloatField{
			String:  "Amount Due in Company Currency",
			Compute: h.AccountInvoice().Methods().ComputeResidual(),
			Stored:  true,
			Depends: []string{"State", "Currency", "InvoiceLines.PriceSubtotal", "Move.Lines.AmountResidual", "Move.Lines.Currency"},
			Help:    "Remaining amount due in the currency of the company."},
		"Payments": models.Many2ManyField{
			RelationModel: h.AccountPayment(),
			JSON:          "payment_ids",
			NoCopy:        true,
			ReadOnly:      true},
		"PaymentMoveLines": models.Many2ManyField{
			String:        "Payment Move Lines",
			RelationModel: h.AccountMoveLine(),
			JSON:          "payment_move_line_ids",
			Compute:       h.AccountInvoice().Methods().ComputePayments(),
			Stored:        true,
			Depends:       []string{"Move.Lines.AmountResidual"}},
		"User": models.Many2OneField{
			String:        "Salesperson",
			RelationModel: h.User(), /*[ track_visibility 'onchange']*/
			/* readonly=true */ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser()
			}},
		"FiscalPosition": models.Many2OneField{
			RelationModel: h.AccountFiscalPosition() /* readonly=true */ /*[ states {'draft': [('readonly']*/ /*[ False)]}]*/},
		"CommercialPartner": models.Many2OneField{
			String:        "Commercial Entity",
			RelationModel: h.Partner(),
			Related:       "Partner.CommercialPartner",
			ReadOnly:      true,
			Help:          "The commercial entity that will be used on Journal Entries for this invoice"},
		"OutstandingCreditsDebitsWidget": models.TextField{
			Compute: h.AccountInvoice().Methods().GetOutstandingInfoJSON()},
		"PaymentsWidget": models.TextField{
			Compute: h.AccountInvoice().Methods().GetPaymentInfoJSON(),
			Depends: []string{"PaymentMoveLines.AmountResidual"}},
		"HasOutstanding": models.BooleanField{
			Compute: h.AccountInvoice().Methods().GetOutstandingInfoJSON()},
	})

	// TODO implement as constraint
	//h.AccountInvoice().AddSQLConstraint("number_uniq", "unique(number, company_id, journal_id, type)",
	//	"Invoice Number must be unique per Company!")

	h.AccountInvoice().Methods().ComputeAmount().DeclareMethod(
		`ComputeAmount`,
		func(rs m.AccountInvoiceSet) m.AccountInvoiceData {
			data := h.AccountInvoice().NewData()
			valueUntaxed := 0.0
			for _, line := range rs.InvoiceLines().Records() {
				valueUntaxed += line.PriceSubtotal()
			}
			data.SetAmountUntaxed(valueUntaxed)
			valueTax := 0.0
			for _, line := range rs.TaxLines().Records() {
				valueTax += line.Amount()
			}
			data.SetAmountTax(valueTax)
			valueTotal := valueUntaxed + valueTax
			data.SetAmountTotal(valueTotal)
			if rs.Currency().IsNotEmpty() && rs.Company().IsNotEmpty() && !rs.Currency().Equals(rs.Company().Currency()) {
				currency := rs.Currency().WithContext("date", rs.DateInvoice().ToDateTime())
				valueTotal = currency.Compute(valueTotal, rs.Company().Currency(), true)
				valueUntaxed = currency.Compute(valueUntaxed, rs.Company().Currency(), true)
			}
			sign := 1.0
			if rs.Type() == "in_refund" || rs.Type() == "out_refund" {
				sign = -1.0
			}
			data.SetAmountTotalCompanySigned(valueTotal * sign)
			data.SetAmountTotalSigned(data.AmountTotal() * sign)
			data.SetAmountUntaxedSigned(valueUntaxed * sign)
			return data
		})

	h.AccountInvoice().Methods().DefaultJournal().DeclareMethod(
		`DefaultJournal`,
		func(rs m.AccountInvoiceSet) m.AccountJournalSet {
			if rs.Env().Context().HasKey("default_journal_id") {
				return h.AccountJournal().Browse(rs.Env(),
					[]int64{rs.Env().Context().GetInteger("default_journal_id")})
			}
			invType := "out_invoice"
			if rs.Env().Context().HasKey("type") {
				invType = rs.Env().Context().GetString("type")
			}
			company := h.User().NewSet(rs.Env()).CurrentUser().Company()
			if rs.Env().Context().HasKey("company_id") {
				company = h.Company().Browse(rs.Env(), []int64{rs.Env().Context().GetInteger("company_id")})
			}
			jType := Type2Journal[invType]
			cond := q.AccountJournal().Type().Equals(jType).And().Company().Equals(company)
			return h.AccountJournal().Search(rs.Env(), cond)
		})

	h.AccountInvoice().Methods().ComputeResidual().DeclareMethod(
		`ComputeResidual`,
		func(rs m.AccountInvoiceSet) m.AccountInvoiceData {
			residual := 0.0
			residualCompanySigned := 0.0
			sign := 1.0
			if rs.Type() == "in_refund" || rs.Type() == "out_refund" {
				sign = -1.0
			}
			for _, line := range rs.Sudo(security.SuperUserID).Move().Lines().Records() {
				if internType := line.Account().InternalType(); internType == "receivable" || internType == "payable" {
					residualCompanySigned += line.AmountResidual()
					if line.Currency().Equals(rs.Currency()) {
						if line.Currency().IsNotEmpty() {
							residual += line.AmountResidualCurrency()
						} else {
							residual += line.AmountResidual()
						}
					} else {
						fromCurrency := line.Company().Currency().WithContext("date", line.Date())
						if line.Currency().IsNotEmpty() {
							fromCurrency = line.Currency().WithContext("date", line.Date())
						}
						residual += fromCurrency.Compute(line.AmountResidual(), rs.Currency(), true)
					}
				}
			}
			data := h.AccountInvoice().NewData()
			data.SetResidualCompanySigned(math.Abs(residualCompanySigned) * sign)
			data.SetResidualSigned(math.Abs(residual) * sign)
			data.SetResidual(math.Abs(residual))
			if nbutils.IsZero(data.Residual(), rs.Currency().Rounding()) {
				data.SetReconciled(true)
			}
			return data
		})

	h.AccountInvoice().Methods().GetOutstandingInfoJSON().DeclareMethod(
		`GetOutstandingInfoJSON`,
		func(rs m.AccountInvoiceSet) m.AccountInvoiceData {
			data := h.AccountInvoice().NewData()
			data.SetOutstandingCreditsDebitsWidget("false")
			if rs.State() != "open" {
				return data
			}
			domain := q.AccountMoveLine().Account().Equals(rs.Account()).
				And().Partner().Equals(h.Partner().NewSet(rs.Env()).FindAccountingPartner(rs.Partner())).
				And().Reconciled().Equals(false).
				And().AmountResidual().NotEquals(0.0)
			var typePayment string
			if strutils.IsIn(rs.Type(), "out_invoice", "in_refund") {
				domain = domain.And().Credit().Greater(0.0).And().Debit().Equals(0.0)
				typePayment = rs.T(`Outstanding credits`)
			} else {
				domain = domain.And().Credit().Equals(0.0).And().Debit().Greater(0.0)
				typePayment = rs.T(`Outstanding debits`)
			}
			lines := h.AccountMoveLine().Search(rs.Env(), domain)
			if lines.IsEmpty() {
				return data
			}
			var infoContent []map[string]interface{}
			for _, line := range lines.Records() {
				// get the outstanding residual value in invoice currency
				var amtToShow float64
				if line.Currency().IsNotEmpty() && line.Currency().Equals(rs.Currency()) {
					amtToShow = math.Abs(line.AmountResidualCurrency())
				} else {
					amtToShow = line.Company().Currency().WithContext("date", line.Date()).Compute(math.Abs(line.AmountResidual()), rs.Currency(), true)
				}
				if nbutils.IsZero(amtToShow, rs.Currency().Rounding()) {
					continue
				}
				curInfoContent := make(map[string]interface{})
				curInfoContent["journal_name"] = line.Move().Name()
				if line.Ref() != "" {
					curInfoContent["journal_name"] = line.Ref()
				}
				curInfoContent["amount"] = amtToShow
				curInfoContent["currency"] = rs.Currency().Symbol()
				curInfoContent["id"] = line.ID()
				curInfoContent["position"] = rs.Currency().Position()
				curInfoContent["digits"] = []int{69, rs.Currency().DecimalPlaces()}
				infoContent = append(infoContent, curInfoContent)
			}
			info := make(map[string]interface{})
			info["title"] = typePayment
			info["outstanding"] = true
			info["invoice_id"] = rs.ID()
			info["content"] = infoContent
			str, err := json.Marshal(info)
			if err != nil {
				panic(rs.T(err.Error()))
			}
			data.SetOutstandingCreditsDebitsWidget(string(str))
			data.SetHasOutstanding(true)
			return data
		})

	h.AccountInvoice().Methods().GetPaymentInfoJSON().DeclareMethod(
		`GetPaymentInfoJSON`,
		func(rs m.AccountInvoiceSet) m.AccountInvoiceData {
			data := h.AccountInvoice().NewData()
			data.SetPaymentsWidget("false")
			if rs.PaymentMoveLines().IsEmpty() {
				return data
			}
			info := map[string]interface{}{
				`title`:       rs.T(`Less Payment`),
				`outstanding`: false,
				`content`:     []map[string]interface{}{}}
			for _, payment := range rs.PaymentMoveLines().Records() {
				var paymentCurrency m.CurrencySet
				var amount float64
				var amountCurrency float64
				if strutils.IsIn(rs.Type(), "out_invoice", "in_refund") {
					targetCurrency := payment.MatchedDebits().Currency()
					for _, p := range payment.MatchedDebits().Records() {
						if p.DebitMove().Subtract(rs.Move().Lines()).IsEmpty() {
							amount += p.Amount()
							amountCurrency += p.AmountCurrency()
						}
						if !p.Currency().Equals(targetCurrency) {
							targetCurrency = h.Currency().NewSet(rs.Env())
						}
					}
					paymentCurrency = targetCurrency
				} else if strutils.IsIn(rs.Type(), "in_invoice", "out_refund") {
					targetCurrency := payment.MatchedCredits().Currency()
					for _, p := range payment.MatchedCredits().Records() {
						if p.CreditMove().Subtract(rs.Move().Lines()).IsEmpty() {
							amount += p.Amount()
							amountCurrency += p.AmountCurrency()
						}
						if !p.Currency().Equals(targetCurrency) {
							targetCurrency = h.Currency().NewSet(rs.Env())
						}
					}
					paymentCurrency = targetCurrency
				}
				// get the payment value in invoice currency
				var amtToShow float64
				if paymentCurrency.IsNotEmpty() && paymentCurrency.Equals(rs.Currency()) {
					amtToShow = amountCurrency
				} else {
					amtToShow = payment.Company().Currency().WithContext("date", payment.Date()).Compute(amount, rs.Currency(), true)
				}
				if nbutils.IsZero(amtToShow, rs.Currency().Rounding()) {
					continue
				}
				paymentRef := payment.Move().Name()
				if payment.Move().Ref() != "" {
					paymentRef += fmt.Sprintf(" (%s)", payment.Move().Ref())
				}
				info[`content`] = append(info[`content`].([]map[string]interface{}), map[string]interface{}{
					`name`:         payment.Name(),
					`journal_name`: payment.Journal().Name(),
					`amount`:       amtToShow,
					`currency`:     rs.Currency().Symbol(),
					`digits`:       []int{69, rs.Currency().DecimalPlaces()},
					`position`:     rs.Currency().Position(),
					`date`:         payment.Date(),
					`payment_id`:   payment.ID(),
					`move_id`:      payment.Move().ID(),
					`ref`:          payment.Ref(),
				})
			}
			str, err := json.Marshal(info)
			if err != nil {
				panic(rs.T(err.Error()))
			}
			data.SetPaymentsWidget(string(str))
			return data
		})

	h.AccountInvoice().Methods().ComputePayments().DeclareMethod(
		`ComputePayments`,
		func(rs m.AccountInvoiceSet) m.AccountInvoiceData {
			var paymentLines m.AccountMoveLineSet
			for _, line := range rs.Move().Lines().Records() {
				for _, rp := range line.MatchedCredits().Records() {
					if rp.CreditMove().IsNotEmpty() {
						paymentLines = paymentLines.Union(rp.CreditMove())
					}
				}
				for _, rp := range line.MatchedDebits().Records() {
					if rp.DebitMove().IsNotEmpty() {
						paymentLines = paymentLines.Union(rp.DebitMove())
					}
				}
			}
			data := h.AccountInvoice().NewData()
			data.SetPaymentMoveLines(paymentLines)
			return data
		})

	var prevent_recursion_with_onchange_fields bool

	h.AccountInvoice().Methods().Create().Extend("",
		func(rs m.AccountInvoiceSet, data m.AccountInvoiceData) m.AccountInvoiceSet {
			if prevent_recursion_with_onchange_fields {
				return rs.Super().Create(data)
			}
			isAnyFieldChanging := func(fields []string) bool {
				for _, field := range fields {
					if data.Has(field) {
						return true
					}
				}
				return false
			}
			onchanges := map[string][]string{
				"OnchangePartner": {"Account", "PaymentTerm", "FiscalPosition", "PartnerBank"},
				"OnchangeJournal": {"Currency"},
			}
			for onchangeMethod, changedFields := range onchanges {
				_ = onchangeMethod
				if isAnyFieldChanging(changedFields) {
					prevent_recursion_with_onchange_fields = true
					invoice := h.AccountInvoice().Create(rs.Env(), data)
					invoice.Call(onchangeMethod)
					for _, field := range changedFields {
						if data.Get(field) == nil && invoice.Get(field) != nil {
							data.Set(field, invoice.Get(field))
						}
					}
					invoice.Unlink()
					prevent_recursion_with_onchange_fields = false
				}
			}
			if data.Account().IsEmpty() {
				panic(rs.T(`Configuration error!\nCould not find any account to create the invoice, are you sure you have a chart of account installed?`))
			}
			invoice := rs.WithContext("mail_create_nolog", true).Super().Create(data)
			if invoice.TaxLines().IsEmpty() {
				hasLines := false
				for _, line := range invoice.InvoiceLines().Records() {
					if line.InvoiceLineTaxes().IsNotEmpty() {
						hasLines = true
					}
				}
				if hasLines {
					invoice.ComputeTaxes()
				}
			}
			return invoice
		})

	h.AccountInvoice().Methods().Write().Extend("",
		func(rs m.AccountInvoiceSet, vals m.AccountInvoiceData) bool {
			preNotReconciled := rs.Filtered(func(rs m.AccountInvoiceSet) bool { return !rs.Reconciled() })
			preReconciled := rs.Subtract(preNotReconciled)
			res := rs.Super().Write(vals)
			reconciled := rs.Filtered(func(rs m.AccountInvoiceSet) bool { return rs.Reconciled() })
			notReconciled := rs.Subtract(reconciled)
			reconciled.Intersect(preReconciled).Filtered(func(rs m.AccountInvoiceSet) bool { return rs.State() == "open" }).ActionInvoicePaid()
			notReconciled.Intersect(preNotReconciled).Filtered(func(rs m.AccountInvoiceSet) bool { return rs.State() == "paid" }).ActionInvoiceReOpen()
			return res
		})

	h.AccountInvoice().Methods().FieldsViewGet().Extend("",
		func(rs m.AccountInvoiceSet, params webdata.FieldsViewGetParams) *webdata.FieldsViewData {
			if !(rs.Env().Context().GetString("active_model") == "res.partner" && rs.Env().Context().Get("acive_ids") != nil) {
				return rs.Super().FieldsViewGet(params)
			}
			partner := h.Partner().BrowseOne(rs.Env(), rs.Env().Context().GetIntegerSlice("active_ids")[0])
			switch params.ViewType {
			case "":
				params.ViewID = "account_invoice_tree"
				params.ViewType = "tree"
			case "form":
				if partner.Supplier() && !partner.Customer() {
					params.ViewID = "account_invoice_supplier_form"
				} else if !partner.Supplier() && partner.Customer() {
					params.ViewID = "account_invoice_form"
				}
			}
			return rs.Super().FieldsViewGet(params)
		})

	h.AccountInvoice().Methods().InvoicePrint().DeclareMethod(
		`Print the invoice and mark it as sent, so that we can see more
			      easily the next step of the workflow`,
		func(rs m.AccountInvoiceSet) *actions.Action {
			rs.EnsureOne()
			rs.SetSent(true)
			// return self.env['report'].get_action(self, 'account.report_invoice') //tovalid
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

	h.AccountInvoice().Methods().ActionInvoiceSent().DeclareMethod(
		`Open a window to compose an email, with the edi invoice template
			      message loaded by default`,
		func(rs m.AccountInvoiceSet) *actions.Action {
			rs.EnsureOne()
			template := views.MakeViewRef("account.email_template_edi_invoice")
			composeForm := views.MakeViewRef("mail.email_compose_message_wizard_form")
			ctx := types.NewContext().
				WithKey("default_model", "account.invoice").
				WithKey("default_res_id", rs.ID()).
				WithKey("default_use_template", !template.IsNull()).
				WithKey("default_composition_mode", "comment").
				WithKey("mark_invoice_as_sent", true).
				WithKey("custom_layout", "account.mail_template_data_notification_email_account_invoice")
			if !template.IsNull() {
				ctx = ctx.WithKey("default_template_id", template.ID())
			}
			return &actions.Action{
				Name:     rs.T("Compose Email"),
				Type:     actions.ActionActWindow,
				ViewMode: "form",
				Model:    "mail.compose.message",
				Views:    []views.ViewTuple{{composeForm.ID(), "form"}},
				View:     composeForm,
				Target:   "new",
				Context:  ctx,
			}
		})

	h.AccountInvoice().Methods().ComputeTaxes().DeclareMethod(
		`Function used in other module to compute the taxes on a fresh invoice created (onchanges did not applied)`,
		func(rs m.AccountInvoiceSet) bool {
			accountInvoiceTaxes := h.AccountInvoiceTax().NewSet(rs.Env())
			for _, invoice := range rs.Records() {
				// Delete non-manual tax lines
				h.AccountInvoiceTax().Search(rs.Env(), q.AccountInvoiceTax().Invoice().Equals(invoice).And().Manual().Equals(false)).Unlink()
				// Generate one tax line per tax, however many invoice lines it's applied to
				taxGrouped := invoice.GetTaxesValues()
				// Create new tax lines
				for _, tax := range taxGrouped {
					accountInvoiceTaxes.Create(tax)
				}
			}
			return true
		})

	h.AccountInvoice().Methods().Unlink().Extend("",
		func(rs m.AccountInvoiceSet) int64 {
			for _, invoice := range rs.Records() {
				if strutils.IsIn(invoice.State(), "draft", "cancel") {
					panic(rs.T(`You cannot delete an invoice which is not draft or cancelled. You should refund it instead.`))
				} else if invoice.MoveName() != "" {
					panic(rs.T(`You cannot delete an invoice after it has been validated (and received a number). You can set it back to "Draft" state and modify its content, then re-confirm it.`))
				}
			}
			return rs.Super().Unlink()
		})

	h.AccountInvoice().Methods().OnchangeInvoiceLines().DeclareMethod(
		`OnchangeInvoiceLines`,
		func(rs m.AccountInvoiceSet) m.AccountInvoiceData {
			taxesGrouped := rs.GetTaxesValues()
			taxLines := rs.TaxLines().Filtered(func(rs m.AccountInvoiceTaxSet) bool { return rs.Manual() })
			for _, tax := range taxesGrouped {
				taxLines.Union(h.AccountInvoiceTax().Create(rs.Env(), tax))
			}
			data := h.AccountInvoice().NewData()
			data.SetTaxLines(taxLines)
			return data
		})

	h.AccountInvoice().Methods().OnchangePartner().DeclareMethod(
		`OnchangePartner`,
		func(rs m.AccountInvoiceSet) m.AccountInvoiceData {
			p := rs.Partner()
			if rs.Company().IsNotEmpty() {
				p = rs.Partner().WithContext("force_company", rs.Company())
			}
			data := h.AccountInvoice().NewData()
			if p.IsNotEmpty() {
				recAccount := p.PropertyAccountReceivable()
				payAccount := p.PropertyAccountPayable()
				if recAccount.IsEmpty() && payAccount.IsEmpty() {
					//action = self.env.ref('account.action_account_config')
					//msg = _('Cannot find a chart of accounts for this company, You should configure it. \nPlease go to Account Configuration.')
					//raise RedirectWarning(msg, action.id, _('Go to the configuration panel'))  //tovalid hexya RedirectWarning
				}
				if strutils.IsIn(rs.Type(), "out_invoice", "out_refund") {
					data.SetAccount(recAccount)
					data.SetPaymentTerm(p.PropertyPaymentTerm())
				} else {
					data.SetAccount(payAccount)
					data.SetPaymentTerm(p.PropertySupplierPaymentTerm())
				}
				deliveryPartner := rs.GetDeliveryPartner()
				data.SetFiscalPosition(h.AccountFiscalPosition().NewSet(rs.Env()).GetFiscalPosition(rs.Partner(), deliveryPartner))
				// If partner has no warning, check its company
				if p.InvoiceWarn() == "no-message" && p.Parent().IsNotEmpty() {
					p = p.Parent()
				}
				// Block if partner only has warning but parent company is blocked
				if p.InvoiceWarn() != "no-message" {
					if p.InvoiceWarn() != "block" && p.Parent().IsNotEmpty() && p.Parent().InvoiceWarn() == "block" {
						p = p.Parent()
					}
					/*
						// tovalid hot to format
						warning = {
								'title': _("Warning for %s") % p.name,
								'message': p.invoice_warn_msg
								}
					*/
					if p.InvoiceWarn() == "block" {
						data.UnsetPartner()
					}
				}
			}
			data.UnsetDateDue()
			if strutils.IsIn(rs.Type(), "in_invoice", "out_refund") {
				banks := p.CommercialPartner().Banks()
				if banks.IsNotEmpty() {
					data.SetPartnerBank(banks.Records()[0])
				}
				// domain = {'partner_bank_id': [('id', 'in', bank_ids.ids)]} // tovalid how to format? (see below)
			}
			/*
			  res = {} // tovalid what do of this?
			  if warning:
			      res['warning'] = warning
			  if domain:
			      res['domain'] = domain
			  return res

			*/
			return data
		})

	h.AccountInvoice().Methods().GetDeliveryPartner().DeclareMethod(
		`GetDeliveryPartner`,
		func(rs m.AccountInvoiceSet) m.PartnerSet {
			rs.EnsureOne()
			return rs.Partner().AddressGet([]string{"delivery"})["delivery"]
		})

	h.AccountInvoice().Methods().OnchangeJournal().DeclareMethod(
		`OnchangeJournal`,
		func(rs m.AccountInvoiceSet) m.AccountInvoiceData {
			data := h.AccountInvoice().NewData()
			if rs.Journal().IsNotEmpty() {
				data.SetCurrency(h.Currency().Coalesce(rs.Journal().Currency(), rs.Journal().Company().Currency()))
			}
			return data
		})

	h.AccountInvoice().Methods().OnchangePaymentTermDateInvoice().DeclareMethod(
		`OnchangePaymentTermDateInvoice`,
		func(rs m.AccountInvoiceSet) m.AccountInvoiceData {
			data := h.AccountInvoice().NewData()
			dateInvoice := rs.DateInvoice()
			if dateInvoice.IsZero() {
				dateInvoice = dates.Now().ToDate()
			}
			if rs.PaymentTerm().IsEmpty() {
				// When no payment term defined
				data.SetDateDue(rs.DateInvoice())
				if dateDue := rs.DateDue(); !dateDue.IsZero() {
					data.SetDateDue(dateDue)
				}
			} else {
				pTerm := rs.PaymentTerm()
				pTermList := pTerm.WithContext("currency_id", rs.Company().Currency()).Compute(1, dateInvoice)
				max := pTermList[0].Date
				for _, line := range pTermList {
					if max.Lower(line.Date) {
						max = line.Date
					}
				}
				data.SetDateDue(max)
			}
			return data
		})

	h.AccountInvoice().Methods().ActionInvoiceDraft().DeclareMethod(
		`ActionInvoiceDraft`,
		func(rs m.AccountInvoiceSet) bool {
			if rs.Filtered(func(rs m.AccountInvoiceSet) bool { return rs.State() != "cancel" }).IsNotEmpty() {
				panic(rs.T(`Invoice must be cancelled in order to reset it to draft.`))
			}
			// go from canceled state to draft state
			rs.Write(h.AccountInvoice().NewData().SetState("draft").UnsetDate())
			// Delete former printed invoice
			/*
				try: // tovalid self.env["report"]
						report_invoice = self.env['report']._get_report_from_name('account.report_invoice')
					except IndexError:
						report_invoice = False
					if report_invoice and report_invoice.attachment:
						for invoice in self:
							with invoice.env.do_in_draft():
								invoice.number, invoice.state = invoice.move_name, 'open'
								attachment = self.env['report']._attachment_stored(invoice, report_invoice)[invoice.id]
							if attachment:
								attachment.unlink()
			*/
			return true
		})

	h.AccountInvoice().Methods().ActionInvoiceProforma2().DeclareMethod(
		`ActionInvoiceProforma2`,
		func(rs m.AccountInvoiceSet) bool {
			if rs.Filtered(func(rs m.AccountInvoiceSet) bool { return rs.State() != "draft" }).IsNotEmpty() {
				panic(rs.T(`Invoice must be a draft in order to set it to Pro-forma.`))
			}
			return rs.Write(h.AccountInvoice().NewData().SetState("proforma2"))
		})

	h.AccountInvoice().Methods().ActionInvoiceOpen().DeclareMethod(
		`ActionInvoiceOpen`,
		func(rs m.AccountInvoiceSet) bool {
			// lots of duplicate calls to action_invoice_open, so we remove those already open
			toOpenInvoices := rs.Filtered(func(rs m.AccountInvoiceSet) bool { return rs.State() != "open" })
			if toOpenInvoices.Filtered(func(rs m.AccountInvoiceSet) bool { return !strutils.IsIn(rs.State(), "proforma2", "draft") }).IsNotEmpty() {
				panic(rs.T(`Invoice must be in draft or Pro-forma state in order to validate it.`))
			}
			toOpenInvoices.ActionDateAssign()
			toOpenInvoices.ActionMoveCreate()
			return toOpenInvoices.InvoiceValidate()
		})

	h.AccountInvoice().Methods().ActionInvoicePaid().DeclareMethod(
		`ActionInvoicePaid`,
		func(rs m.AccountInvoiceSet) bool {
			// lots of duplicate calls to action_invoice_paid, so we remove those already paid
			toPayInvoices := rs.Filtered(func(rs m.AccountInvoiceSet) bool { return rs.State() != "paid" })
			if toPayInvoices.Filtered(func(rs m.AccountInvoiceSet) bool { return rs.State() != "open" }).IsNotEmpty() {
				panic(rs.T(`Invoice must be validated in order to set it to register payment.`))
			}
			if toPayInvoices.Filtered(func(rs m.AccountInvoiceSet) bool { return !rs.Reconciled() }).IsNotEmpty() {
				panic(rs.T(`You cannot pay an invoice which is partially paid. You need to reconcile payment entries first.`))
			}
			if toPayInvoices.IsNotEmpty() {
				toPayInvoices.SetState("paid")
			}
			return true
		})

	h.AccountInvoice().Methods().ActionInvoiceReOpen().DeclareMethod(
		`ActionInvoiceReOpen`,
		func(rs m.AccountInvoiceSet) bool {
			if rs.Filtered(func(rs m.AccountInvoiceSet) bool { return rs.State() != "paid" }).IsNotEmpty() {
				panic(rs.T(`Invoice must be paid in order to set it to register payment.`))
			}
			if rs.IsNotEmpty() {
				rs.SetState("open")
			}
			return true
		})

	h.AccountInvoice().Methods().ActionInvoiceCancel().DeclareMethod(
		`ActionInvoiceCancel`,
		func(rs m.AccountInvoiceSet) bool {
			if rs.Filtered(func(rs m.AccountInvoiceSet) bool { return strutils.IsIn(rs.State(), "proforma2", "draft", "open") }).IsNotEmpty() {
				panic(rs.T(`Invoice must be in draft, Pro-forma or open state in order to be cancelled.`))
			}
			return rs.ActionCancel()
		})

	h.AccountInvoice().Methods().GetFormviewId().Extend(
		"Update form view id of action to open the invoice",
		func(rs m.AccountInvoiceSet) string {
			if strutils.IsIn(rs.Type(), "in_invoice", "in_refund") {
				return views.MakeViewRef("account.invoice_supplier_form").ID()
			} else {
				return views.MakeViewRef(`account.invoice_form`).ID()
			}
		})

	h.AccountInvoice().Methods().PrepareTaxLineVals().DeclareMethod(
		`PrepareTaxLineVals Prepare values to create an account.invoice.tax line
        The line parameter is an account.invoice.line, and the
        tax parameter is the output of account.tax.compute_all().
        `,
		func(rs m.AccountInvoiceSet, line m.AccountInvoiceLineSet, tax accounttypes.AppliedTaxData) m.AccountInvoiceTaxData {
			vals := h.AccountInvoiceTax().NewData().
				SetInvoice(rs).
				SetName(tax.Name).
				SetTax(h.AccountTax().BrowseOne(rs.Env(), tax.ID)).
				SetAmount(tax.Amount).
				SetSequence(int64(tax.Sequence))
			if tax.Analytic {
				vals.SetAccountAnalytic(line.AccountAnalytic())
			}
			if strutils.IsIn(rs.Type(), "out_invoice", "in_invoice") {
				vals.SetAccount(h.AccountAccount().Coalesce(h.AccountAccount().BrowseOne(rs.Env(), tax.AccountID), line.Account()))
			} else {
				vals.SetAccount(h.AccountAccount().Coalesce(h.AccountAccount().BrowseOne(rs.Env(), tax.RefundAccountID), line.Account()))
			}
			// If the taxes generate moves on the same financial account as the invoice line,
			//	propagate the analytic account from the invoice line to the tax line.
			//	This is necessary in situations were (part of) the taxes cannot be reclaimed,
			//	to ensure the tax move is allocated to the proper analytic account.
			if vals.AccountAnalytic().IsEmpty() && line.AccountAnalytic().IsNotEmpty() && vals.Account().Equals(line.Account()) {
				vals.SetAccountAnalytic(line.AccountAnalytic())
			}
			h.AccountInvoiceTax().NewData()
			return vals
		})

	h.AccountInvoice().Methods().GetTaxesValues().DeclareMethod(
		`GetTaxesValues`,
		func(rs m.AccountInvoiceSet) map[string]m.AccountInvoiceTaxData {
			taxGrouped := make(map[string]m.AccountInvoiceTaxData)
			for _, line := range rs.InvoiceLines().Records() {
				priceUnit := line.PriceUnit() * (1 - line.Discount()/100)
				_, _, _, taxes := line.InvoiceLineTaxes().ComputeAll(priceUnit, rs.Currency(), line.Quantity(), line.Product(), rs.Partner())
				for _, t := range taxes {
					val := rs.PrepareTaxLineVals(line, t)
					key := h.AccountTax().BrowseOne(rs.Env(), t.ID).GetGroupingKey(val)
					if data, ok := taxGrouped[key]; !ok {
						taxGrouped[key] = val
					} else {
						taxGrouped[key].SetAmount(data.Amount() + val.Amount())
						taxGrouped[key].SetBase(data.Base() + val.Base())
					}
				}
			}
			return taxGrouped
		})

	h.AccountInvoice().Methods().RegisterPayment().DeclareMethod(
		`RegisterPayment Reconcile payable/receivable lines from the invoice with payment_line`,
		func(rs m.AccountInvoiceSet, paymentLine m.AccountMoveLineSet, writeOffAccount m.AccountAccountSet,
			writeOffJournal m.AccountJournalSet) m.AccountMoveLineSet {
			lineToReconcile := h.AccountMoveLine().NewSet(rs.Env())
			for _, inv := range rs.Records() {
				lineToReconcile = lineToReconcile.Union(inv.Move().Lines().Filtered(func(rs m.AccountMoveLineSet) bool {
					return !rs.Reconciled() && strutils.IsIn(rs.Account().InternalType(), "payable", "receivable")
				}))
			}
			return lineToReconcile.Union(paymentLine).Reconcile(writeOffAccount, writeOffJournal)
		})

	h.AccountInvoice().Methods().AssignOutstandingCredit().DeclareMethod(
		`AssignOutstandingCredit`,
		func(rs m.AccountInvoiceSet, creditAML m.AccountMoveLineSet) m.AccountMoveLineSet {
			rs.EnsureOne()
			if creditAML.Currency().IsEmpty() && !rs.Currency().Equals(rs.Company().Currency()) {
				creditAML.WithContext("allow_amount_currency", true).Write(
					h.AccountMoveLine().NewData().SetCurrency(rs.Currency()).
						SetAmountCurrency(rs.Company().Currency().WithContext("date", creditAML.Date()).Compute(creditAML.Balance(), rs.Currency(), true)))
			}
			if creditAML.Payment().IsNotEmpty() {
				creditAML.Payment().Write(h.AccountPayment().NewData().SetInvoices(rs))
			}
			return rs.RegisterPayment(creditAML, h.AccountAccount().NewSet(rs.Env()), h.AccountJournal().NewSet(rs.Env()))
		})

	h.AccountInvoice().Methods().ActionDateAssign().DeclareMethod(
		`ActionDateAssign`,
		func(rs m.AccountInvoiceSet) bool {
			for _, inv := range rs.Records() {
				//Here the onchange will automatically write to the database
				inv.OnchangePaymentTermDateInvoice()
			}
			return true
		})

	h.AccountInvoice().Methods().FinalizeInvoiceMoveLines().DeclareMethod(
		`FinalizeInvoiceMoveLines is a hook method to be overridden in additional modules to verify and
		possibly alter the move lines to be created by an invoice, for special cases.`,
		func(rs m.AccountInvoiceSet, moveLines []m.AccountMoveLineData) []m.AccountMoveLineData {
			return moveLines
		})

	h.AccountInvoice().Methods().GetCurrencyRateDate().DeclareMethod(
		`GetCurrencyRateDate`,
		func(rs m.AccountInvoiceSet) dates.Date {
			if !rs.Date().IsZero() {
				return rs.Date()
			}
			return rs.DateInvoice()
		})

	h.AccountInvoice().Methods().ComputeInvoiceTotals().DeclareMethod(
		`ComputeInvoiceTotals`,
		func(rs m.AccountInvoiceSet, companyCurrency m.CurrencySet, invoiceMoveLines []m.AccountInvoiceLineData) (float64, float64, []m.AccountInvoiceLineData) {
			//@api.multi
			/*def compute_invoice_totals(self, company_currency, invoice_move_lines):
						for line in invoice_move_lines:
							if self.currency_id != company_currency:
								currency = self.currency_id.with_context(date=self._get_currency_rate_date() or fields.Date.context_today(self))
								if not (line.get('currency_id') and line.get('amount_currency')):
									line['currency_id'] = currency.id
									line['amount_currency'] = currency.round(line['price'])
									line['price'] = currency.compute(line['price'], company_currency)
							else:
								line['currency_id'] = False
								line['amount_currency'] = False
								line['price'] = self.currency_id.round(line['price'])
							if self.type in ('out_invoice', 'in_refund'):
								total += line['price']
								total_currency += line['amount_currency'] or line['price']
								line['price'] = - line['price']
							else:
								total -= line['price']
								total_currency -= line['amount_currency'] or line['price']
			        	return total, total_currency, invoice_move_lines*/
			//total := 0.0
			//totalCurrency := 0.0
			//for _, line := range invoiceMoveLines {
			//	if !rs.Currency().Equals(companyCurrency) {
			//		lineData := h.AccountMoveLine().NewData()
			//		var dateVal dates.Date
			//		if dateVal = rs.GetCurrencyRateDate(); dateVal.IsZero() {
			//			dateVal = dates.Now().ToDate()
			//		}
			//		currency := rs.Currency().WithContext("date", dateVal)
			//		if !(line.Currency().IsEmpty() && line.AmountCurrency() == 0.0) {
			//			lineData.SetCurrency(currency).SetAmountCurrency(currency.Round(line.Price())) //tovalid field Price missing
			//		}
			//	}
			//}
			return 0, 0, nil
		})

	h.AccountInvoice().Methods().InvoiceLineMoveLineGet().DeclareMethod(
		`InvoiceLineMoveLineGet`,
		func(rs m.AccountInvoiceSet) []m.AccountInvoiceLineData {
			//@api.model
			/*def invoice_line_move_line_get(self):
			  res = []
			  for line in self.invoice_line_ids:
			      if line.quantity==0:
			          continue
			      tax_ids = []
			      for tax in line.invoice_line_tax_ids:
			          tax_ids.append((4, tax.id, None))
			          for child in tax.children_tax_ids:
			              if child.type_tax_use != 'none':
			                  tax_ids.append((4, child.id, None))
			      analytic_tag_ids = [(4, analytic_tag.id, None) for analytic_tag in line.analytic_tag_ids]

			      move_line_dict = {  // tovalid data type?
			          'invl_id': line.id,
			          'type': 'src',
			          'name': line.name.split('\n')[0][:64],
			          'price_unit': line.price_unit,
			          'quantity': line.quantity,
			          'price': line.price_subtotal,
			          'account_id': line.account_id.id,
			          'product_id': line.product_id.id,
			          'uom_id': line.uom_id.id,
			          'account_analytic_id': line.account_analytic_id.id,
			          'tax_ids': tax_ids,
			          'invoice_id': self.id,
			          'analytic_tag_ids': analytic_tag_ids
			      }
			      if line['account_analytic_id']:
			          move_line_dict['analytic_line_ids'] = [(0, 0, line._get_analytic_line())]
			      res.append(move_line_dict)
			  return res

			*/
			var res []m.AccountInvoiceLineData
			//for _, line := range rs.InvoiceLines().Records() {
			//	data := h.AccountTax().NewData()
			//	if line.Quantity() == 0.0 {
			//		continue
			//	}
			//	for _, tax := range line.InvoiceLineTaxes().Records() {
			//	}
			//}
			return res
		})

	h.AccountInvoice().Methods().TaxLineMoveLineGet().DeclareMethod(
		`TaxLineMoveLineGet`,
		func(rs m.AccountInvoiceSet) []m.AccountInvoiceLineData {
			//@api.model
			/*def tax_line_move_line_get(self):
			  res = []
			  # keep track of taxes already processed
			  done_taxes = []
			  # loop the invoice.tax.line in reversal sequence
			  for tax_line in sorted(self.tax_line_ids, key=lambda x: -x.sequence):
			      if tax_line.amount:
			          tax = tax_line.tax_id
			          if tax.amount_type == "group":
			              for child_tax in tax.children_tax_ids:
			                  done_taxes.append(child_tax.id)
			          res.append({ //tovalid data type?
			              'invoice_tax_line_id': tax_line.id,
			              'tax_line_id': tax_line.tax_id.id,
			              'type': 'tax',
			              'name': tax_line.name,
			              'price_unit': tax_line.amount,
			              'quantity': 1,
			              'price': tax_line.amount,
			              'account_id': tax_line.account_id.id,
			              'account_analytic_id': tax_line.account_analytic_id.id,
			              'invoice_id': self.id,
			              'tax_ids': [(6, 0, list(done_taxes))] if tax_line.tax_id.include_base_amount else []
			          })
			          done_taxes.append(tax.id)
			  return res

			*/
			return []m.AccountInvoiceLineData{}
		})

	h.AccountInvoice().Methods().InvLineCharacteristicHashcode().DeclareMethod(
		`InvLineCharacteristicHashcode Overridable hashcode generation for invoice lines. Lines having the same hashcode
			  will be grouped together if the journal has the 'group line' option. Of course a module
			  can add fields to invoice lines that would need to be tested too before merging lines
			  or not.`,
		func(rs m.AccountInvoiceSet, invoiceLine m.AccountMoveLineData) string {
			return fmt.Sprintf(`%d-%v-%d-%d-%d-%s-%v`,
				invoiceLine.Account().ID(),
				invoiceLine.Taxes().Ids(),
				invoiceLine.TaxLine().ID(),
				invoiceLine.Product().ID(),
				invoiceLine.AnalyticAccount().ID(),
				invoiceLine.DateMaturity().String(),
				invoiceLine.AnalyticTags().Ids())
		})

	h.AccountInvoice().Methods().GroupLines().DeclareMethod(
		`GroupLines Merge account move lines (and hence analytic lines) if invoice line hashcodes are equals`,
		func(rs m.AccountInvoiceSet, iml []m.AccountInvoiceLineData, lines []m.AccountMoveLineData) []m.AccountMoveLineData {
			var datas []m.AccountMoveLineData
			if rs.Journal().GroupInvoiceLines() {
				line2 := make(map[string]m.AccountMoveLineData)
				for _, line := range lines {
					tmp := rs.InvLineCharacteristicHashcode(line)
					if data, ok := line2[tmp]; ok {
						am := data.Debit() - data.Credit() + (line.Debit() - line.Credit())
						if am > 0 {
							data.SetDebit(am)
						} else if am < 0 {
							data.SetCredit(am)
						}
						data.SetAmountCurrency(data.AmountCurrency() + line.AmountCurrency())
						data.SetAnalyticLines(data.AnalyticLines().Union(line.AnalyticLines()))
						if qty := line.Quantity(); qty != 0.0 {
							data.SetQuantity(data.Quantity() + qty)
						}
					} else {
						line2[tmp] = line
					}
				}
				for _, val := range line2 {
					datas = append(datas, val)
				}
			}
			return datas
		})

	h.AccountInvoice().Methods().ActionMoveCreate().DeclareMethod(
		`ActionMoveCreate creates invoice related analytics and financial move lines`,
		func(rs m.AccountInvoiceSet) bool {
			for _, inv := range rs.Records() {
				if inv.Journal().EntrySequence().IsEmpty() {
					panic(rs.T(`Please define sequence on the journal related to this invoice.`))
				}
				if inv.InvoiceLines().IsEmpty() {
					panic(rs.T(`Please create some invoice lines.`))
				}
				if inv.Move().IsNotEmpty() {
					continue
				}
				ctx := rs.Env().Context().WithKey("lang", inv.Partner().Lang())
				if inv.DateInvoice().IsZero() {
					inv.WithNewContext(ctx).Write(h.AccountInvoice().NewData().SetDateInvoice(dates.Now().ToDate()))
				}
				companyCurrency := inv.Company().Currency()
				// create move lines (one per invoice line + eventual taxes and analytic lines)
				iml := append(inv.InvoiceLineMoveLineGet(), inv.TaxLineMoveLineGet()...)
				diffCurrency := !inv.Currency().Equals(companyCurrency)
				// create one move line for the total and possibly adjust the other lines amount
				total, totalCurrency, newIml := inv.WithNewContext(ctx).ComputeInvoiceTotals(companyCurrency, iml)
				iml = newIml
				name := "/"
				if inv.Name() != "" {
					name = inv.Name()
				}
				// FIXME
				fmt.Println(diffCurrency, total, totalCurrency, name)
				if inv.PaymentTerm().IsNotEmpty() {
					/*
						totlines = inv.with_context(ctx).payment_term_id.with_context(currency_id=company_currency.id).compute(total, inv.date_invoice)[0]
						res_amount_currency = total_currency
						ctx['date'] = inv._get_currency_rate_date()
						for i, t in enumerate(totlines):
							if inv.currency_id != company_currency:
								amount_currency = company_currency.with_context(ctx).compute(t[1], inv.currency_id)
							else:
								amount_currency = False

							# last line: add the diff
							res_amount_currency -= amount_currency or 0
							if i + 1 == len(totlines):
								amount_currency += res_amount_currency

							iml.append({ //tovalid fields missing
								'type': 'dest',
								'name': name,
								'price': t[1],
								'account_id': inv.account_id.id,
								'date_maturity': t[0],
								'amount_currency': diff_currency and amount_currency,
								'currency_id': diff_currency and inv.currency_id.id,
								'invoice_id': inv.id
							})
					*/
				} else {
					/*
						iml.append({ //tovalid fields missing
							'type': 'dest',
							'name': name,
							'price': total,
							'account_id': inv.account_id.id,
							'date_maturity': inv.date_due,
							'amount_currency': diff_currency and total_currency,
							'currency_id': diff_currency and inv.currency_id.id,
							'invoice_id': inv.id
						})
					*/
				}
				part := rs.Partner().FindAccountingPartner(inv.Partner())
				var line []m.AccountMoveLineData
				for _, l := range iml {
					line = append(line, rs.LineGetConvert(l, part))
				}
				line = inv.FinalizeInvoiceMoveLines(inv.GroupLines(iml, line))
				journal := inv.Journal().WithNewContext(ctx)
				date := inv.DateInvoice()
				if val := inv.Date(); !val.IsZero() {
					date = val
				}
				lines := h.AccountMoveLine().NewSet(rs.Env())
				for _, l := range line {
					lines = lines.Union(h.AccountMoveLine().Create(rs.Env(), l))
				}
				data := h.AccountMove().NewData().
					SetRef(inv.Reference()).
					SetLines(lines).
					SetJournal(journal).
					SetDate(date).
					SetNarration(inv.Comment())
				ctx = ctx.WithKey("company_id", inv.Company().ID()).WithKey("invoice_id", inv.ID())
				ctxNolang := ctx.Copy().WithKey("lang", "")
				move := h.AccountMove().NewSet(rs.Env()).WithNewContext(ctxNolang).Create(data)
				// Pass invoice in context in method post: used if you want to get the same
				// account move reference when creating the same invoice after a cancelled one:
				move.Post()
				//  make the invoice point to that move
				vals := h.AccountInvoice().NewData().
					SetMove(move).
					SetDate(date).
					SetMoveName(move.Name())
				inv.WithNewContext(ctx).Write(vals)
			}
			return true
		})

	h.AccountInvoice().Methods().InvoiceValidate().DeclareMethod(
		`InvoiceValidate`,
		func(rs m.AccountInvoiceSet) bool {
			for _, invoice := range rs.Records() {
				// refuse to validate a vendor bill/refund if there already exists one with the same reference for the same partner,
				// because it's probably a double encoding of the same bill/refund
				if strutils.IsIn(invoice.Type(), "in_invoice", "in_refund") {
					panic(rs.T(`Duplicated vendor reference detected. You probably encoded twice the same vendor bill/refund.`))
				}
			}
			return rs.Write(h.AccountInvoice().NewData().SetState("open"))
		})

	h.AccountInvoice().Methods().LineGetConvert().DeclareMethod(
		`LineGetConvert`,
		func(rs m.AccountInvoiceSet, line m.AccountInvoiceLineData, partner m.PartnerSet) m.AccountMoveLineData {
			//@api.model
			data := h.AccountMoveLine().NewData()
			//data.SetDateMaturity(line.DateMaturity()) //tovalid line.DateMaturity missing
			data.SetPartner(partner)
			data.SetName(line.Name())
			//if line.Price() > 0 {  //tovalid line.Price missing
			//	data.SetDebit(line.Price)
			//} else {
			//	data.SetCredit(line.Price())
			//}
			/*def line_get_convert(self, line, part):
			  return {
			      'account_id': line['account_id'],
			      'analytic_line_ids': line.get('analytic_line_ids', []),
			      'amount_currency': line['price'] > 0 and abs(line.get('amount_currency', False)) or -abs(line.get('amount_currency', False)),
			      'currency_id': line.get('currency_id', False),
			      'quantity': line.get('quantity', 1.00),
			      'product_id': line.get('product_id', False),
			      'product_uom_id': line.get('uom_id', False),
			      'analytic_account_id': line.get('account_analytic_id', False),
			      'invoice_id': line.get('invoice_id', False),
			      'tax_ids': line.get('tax_ids', False),
			      'tax_line_id': line.get('tax_line_id', False),
			      'analytic_tag_ids': line.get('analytic_tag_ids', False),
			  }

			*/
			return h.AccountMoveLine().NewData()
		})

	h.AccountInvoice().Methods().ActionCancel().DeclareMethod(
		`ActionCancel`,
		func(rs m.AccountInvoiceSet) bool {
			moves := h.AccountMove().NewSet(rs.Env())
			for _, inv := range rs.Records() {
				if inv.PaymentMoveLines().IsNotEmpty() {
					panic(rs.T(`You cannot cancel an invoice which is partially paid. You need to unreconcile related payment entries first.'`))
				}
				if inv.Move().IsNotEmpty() {
					moves = moves.Union(inv.Move())
				}
			}
			// First, set the invoices as cancelled and detach the move ids
			rs.Write(h.AccountInvoice().NewData().SetState("cancel").SetMove(h.AccountMove().NewSet(rs.Env())))
			if moves.IsEmpty() {
				return true
			}
			// second, invalidate the move(s)
			moves.ButtonCancel()
			// delete the move this invoice was pointing to
			// Note that the corresponding move_lines and move_reconciles
			// will be automatically deleted too
			moves.Unlink()
			return true
		})

	h.AccountInvoice().Methods().NameGet().Extend("",
		func(rs m.AccountInvoiceSet) string {
			types := map[string]string{
				"out_invoice": rs.T(`Invoice`),
				"in_invoice":  rs.T(`Vendor Bill`),
				"out_refund":  rs.T(`Refund`),
				"in_refund":   rs.T(`Vendor Refund`),
			}
			result := rs.Number()
			if result == "" {
				result = types[rs.Type()]
			}
			if name := rs.Name(); name != "" {
				result = result + " " + name
			}
			return result
		})

	h.AccountInvoice().Methods().SearchByName().Extend("",
		func(rs m.AccountInvoiceSet, name string, op operator.Operator, additionalCond q.AccountInvoiceCondition, limit int) m.AccountInvoiceSet {
			//@api.model
			/*def name_search(self, name, args=None, operator='ilike', limit=100):
			  args = args or []
			  recs = self.browse()
			  if name:
			      recs = self.search([('number', '=', name)] + args, limit=limit)
			  if not recs:
			      recs = self.search([('name', operator, name)] + args, limit=limit)
			  return recs.name_get()	tovalid return tuple or string as recordset ?

			*/
			return rs.Super().SearchByName(name, op, additionalCond, limit)
		})

	h.AccountInvoice().Methods().GetRefundCommonFields().DeclareMethod(
		`GetRefundCommonFields`,
		func(rs m.AccountInvoiceSet) []models.FieldNamer {
			return models.ConvertToFieldNameSlice([]string{"Partner", "PaymentTerm", "Account", "Currency", "Journal"})
		})

	h.AccountInvoice().Methods().GetRefundPrepareFields().DeclareMethod(
		`GetRefundPrepareFields`,
		func(rs m.AccountInvoiceSet) []models.FieldNamer {
			return models.ConvertToFieldNameSlice([]string{"Name", "Reference", "Comment", "DateDue"})
		})

	h.AccountInvoice().Methods().GetRefundModifyReadFields().DeclareMethod(
		`GetRefundModifyReadFields`,
		func(rs m.AccountInvoiceSet) []models.FieldNamer {
			readFields := models.ConvertToFieldNameSlice([]string{"Type", "Number", "InvoiceLines", "TaxLines", "Date"})
			return append(rs.GetRefundCommonFields(), readFields...)
		})

	h.AccountInvoice().Methods().GetRefundCopyFields().DeclareMethod(
		`GetRefundCopyFields`,
		func(rs m.AccountInvoiceSet) []models.FieldNamer {
			copyFields := models.ConvertToFieldNameSlice([]string{"Company", "User", "FiscalPosition"})
			return append(rs.GetRefundCommonFields(), copyFields...)
		})

	h.AccountInvoice().Methods().PrepareRefund().DeclareMethod(
		`PrepareRefund Prepare the dict of values to create the new refund from the invoice.
						      This method may be overridden to implement custom
						      refund generation (making sure to call super() to establish
						      a clean extension chain).

						      :param record invoice: invoice to refund
						      :param string date_invoice: refund creation date from the wizard
						      :param integer date: force date from the wizard
						      :param string description: description of the refund from the wizard
						      :param integer journal_id: account.journal from the wizard
						      :return: dict of value to create() the refund`,
		func(rs m.AccountInvoiceSet, invoice m.AccountInvoiceSet, dateInvoice, date dates.Date,
			description string, journal m.AccountJournalSet) m.AccountInvoiceData {

			values := h.AccountInvoice().NewData()
			for _, field := range rs.GetRefundCopyFields() {
				/*
						  if invoice._fields[field].type == 'many2one': //tovalid invoice._fields ?
					          values[field] = invoice[field].id
					      else:
					          values[field] = invoice[field] or False
				*/
				// FIXME
				fmt.Println(field)
			}
			values.SetInvoiceLines(invoice.InvoiceLines())
			taxLines := invoice.TaxLines()
			values.SetTaxLines(taxLines)
			var journ m.AccountJournalSet
			switch {
			case journal.IsNotEmpty():
				journ = journal
			case invoice.Type() == "in_invoice":
				journ = h.AccountJournal().Search(rs.Env(), q.AccountJournal().Type().Equals("purchase")).Limit(1)
			default:
				journ = h.AccountJournal().Search(rs.Env(), q.AccountJournal().Type().Equals("sale")).Limit(1)
			}
			values.SetJournal(journ).
				SetType(accounttypes.TYPE2REFUND[invoice.Type()]).
				SetType("draft").
				SetNumber("").
				SetOrigin(invoice.Number()).
				SetRefundInvoice(invoice)

			if !dateInvoice.IsZero() {
				values.SetDateInvoice(dateInvoice)
			} else {
				values.SetDateInvoice(dates.Today())
			}
			if !date.IsZero() {
				values.SetDate(date)
			}
			if description != "" {
				values.SetName(description)
			}
			return values
		})

	h.AccountInvoice().Methods().Refund().DeclareMethod(
		`Refund`,
		func(rs m.AccountInvoiceSet, dateInvoice, date dates.Date,
			description string, journal m.AccountJournalSet) m.AccountInvoiceSet {
			var CreatedInvoices m.AccountInvoiceSet
			invoiceType := map[string]string{
				"out_invoice": rs.T("customer invoices refund"),
				"in_invoice":  rs.T("vendor bill refund"),
			}
			baseMessage := rs.T("This %s has been created from: <a href=# data-oe-model=account.invoice data-oe-id=%d>%s</a>")
			for _, invoice := range rs.Records() {
				// create the new invoice
				values := rs.PrepareRefund(invoice, dateInvoice, date, description, journal)
				refundInvoice := rs.Create(values)

				message := fmt.Sprintf(baseMessage, invoiceType[invoice.Type()], invoice.ID(), invoice.Number())
				// FIXME
				fmt.Println(message)
				// refund_invoice.MessagePost(message) //tovalid MessagePost func missing?
				CreatedInvoices = CreatedInvoices.Union(refundInvoice)
			}
			return rs.Union(CreatedInvoices)
		})

	h.AccountInvoice().Methods().PayAndReconcile().DeclareMethod(
		`PayAndReconcileCreate and post an account.payment for the invoice self, which creates a journal entry that reconciles the invoice.

			      :param pay_journal: journal in which the payment entry will be created
			      :param pay_amount: amount of the payment to register, defaults to the residual of the invoice
			      :param date: payment date, defaults to fields.Date.context_today(self)
			      :param writeoff_acc: account in which to create a writeoff if pay_amount < self.residual, so that the invoice is fully paid`,
		func(rs m.AccountInvoiceSet, payJournal m.AccountJournalSet, payAmount float64,
			date dates.Date, writeoffAcc m.AccountAccountSet) bool {
			//@api.multi
			if rs.Len() != 1 {
				panic(rs.T("Can only pay one invoice at a time"))
			}
			paymentType := "outbound"
			var paymentMethod m.AccountPaymentMethodSet
			var journalPaymentMethods m.AccountPaymentMethodSet
			if strutils.IsIn(rs.Type(), "out_invoice", "in_refund") {
				paymentType = "inbound"
				paymentMethod = h.AccountPaymentMethod().NewSet(rs.Env()).GetRecord(`account_account_payment_method_manual_in`) //tovalid ??? recordModel not found??? wtf
				journalPaymentMethods = payJournal.InboundPaymentMethods()
			} else {
				paymentMethod = h.AccountPaymentMethod().NewSet(rs.Env()).GetRecord(`account_account_payment_method_manual_out`) //tovalid ??? recordModel not found??? wtf
				journalPaymentMethods = payJournal.OutboundPaymentMethods()
			}
			if paymentMethod.Intersect(journalPaymentMethods).Len() == 0 {
				panic(rs.T(`No appropriate payment method enabled on journal %s`, payJournal.Name()))
			}
			communication := rs.Number()
			if strutils.IsIn(rs.Type(), "in_invoice", "in_refund") {
				communication = rs.Reference()
			}
			if rs.Origin() != "" {
				communication = fmt.Sprintf("%s (%s)", communication, rs.Origin())
			}
			data := h.AccountPayment().NewData().
				SetInvoices(rs).
				SetCommunication(communication).
				SetPartner(rs.Partner()).
				SetJournal(payJournal).
				SetPaymentType(paymentType).
				SetPaymentMethod(paymentMethod).
				SetAmount(rs.Residual()).
				SetPaymentDate(dates.Today()).
				SetPartnerType("supplier").
				SetPaymentDifferenceHandling("open").
				SetWriteoffAccount(h.AccountAccount().NewSet(rs.Env()))
			if payAmount != 0.0 {
				data.SetAmount(payAmount)
			}
			if !date.IsZero() {
				data.SetPaymentDate(date)
			}
			if strutils.IsIn(rs.Type(), "out_invoice", "out_refund") {
				data.SetPartnerType("customer")
			}
			if writeoffAcc.IsNotEmpty() {
				data.SetPaymentDifferenceHandling("reconcile")
				data.SetWriteoffAccount(writeoffAcc)
			}
			return true
		})

	h.AccountInvoice().Methods().GetTaxAmountByGroup().DeclareMethod(
		`GetTaxAmountByGroup`,
		func(rs m.AccountInvoiceSet) []accounttypes.TaxGroup {
			//@api.multi
			/*def _get_tax_amount_by_group(self):  //tovalid dont understand
			  self.ensure_one()
			  res = {}
			  currency = self.currency_id or self.company_id.currency_id
			  for line in self.tax_line_ids:
			      res.setdefault(line.tax_id.tax_group_id, 0.0)
			      res[line.tax_id.tax_group_id] += line.amount
			  res = sorted(res.items(), key=lambda l: l[0].sequence)
			  res = map(lambda l: (l[0].name, l[1]), res)
			  return res
			*/
			return []accounttypes.TaxGroup{}
		})

	h.AccountInvoiceLine().DeclareModel()
	h.AccountInvoiceLine().SetDefaultOrder("Invoice", "Sequence", "ID")

	h.AccountInvoiceLine().AddFields(map[string]models.FieldDefinition{
		"Name": models.TextField{
			String:   "Description",
			Required: true},
		"Origin": models.CharField{
			String: "Source Document",
			Help:   "Reference of the document that produced this invoice."},
		"Sequence": models.IntegerField{
			Default: models.DefaultValue(10),
			Help:    "Gives the sequence of this line when displaying the invoice."},
		"Invoice": models.Many2OneField{
			String:        "Invoice Reference",
			RelationModel: h.AccountInvoice(),
			OnDelete:      models.Cascade,
			Index:         true},
		"Uom": models.Many2OneField{
			String:        "Unit of Measure",
			RelationModel: h.ProductUom(),
			OnDelete:      models.SetNull,
			Index:         true,
			OnChange:      h.AccountInvoiceLine().Methods().OnchangeUom()},
		"Product": models.Many2OneField{
			String:        "Product",
			RelationModel: h.ProductProduct(),
			OnDelete:      models.Restrict,
			Index:         true,
			OnChange:      h.AccountInvoiceLine().Methods().OnchangeProduct()},
		"Account": models.Many2OneField{
			String:        "Account",
			RelationModel: h.AccountAccount(),
			Required:      true,
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Default: func(env models.Environment) interface{} {
				if !env.Context().HasKey("journal_id") {
					return h.AccountJournal().NewSet(env)
				}
				journal := h.AccountJournal().Browse(env, []int64{env.Context().GetInteger("journal_id")})
				if strutils.IsIn(env.Context().GetString("type"), "out_invoice", "in_refund") {
					return journal.DefaultCreditAccount()
				}
				return journal.DefaultDebitAccount()
			},
			Help:     "The income or expense account related to the selected product.",
			OnChange: h.AccountInvoiceLine().Methods().OnchangeAccount()},
		"PriceUnit": models.FloatField{
			String:   "Unit Price",
			Required: true,
			Digits:   decimalPrecision.GetPrecision("Product Price")},
		"PriceSubtotal": models.FloatField{
			String:  "Amount",
			Stored:  true,
			Compute: h.AccountInvoiceLine().Methods().ComputePrice(),
			Depends: []string{"PriceUnit", "Discount", "InvoiceLineTaxes", "Quantity", "Product", "Invoice.Partner",
				"Invoice.Currency", "Invoice.Company", "Invoice.DateInvoice"}},
		"PriceSubtotalSigned": models.FloatField{
			String:  "Amount Signed", /*[ currency_field 'company_currency_id']*/
			Stored:  true,
			Compute: h.AccountInvoiceLine().Methods().ComputePrice(),
			Depends: []string{"PriceUnit", "Discount", "InvoiceLineTaxes", "Quantity", "Product", "Invoice.Partner",
				"Invoice.Currency", "Invoice.Company", "Invoice.DateInvoice"},
			Help: "Total amount in the currency of the company, negative for credit notes."},
		"Quantity": models.FloatField{
			Digits:   decimalPrecision.GetPrecision("Product Unit of Measure"),
			Required: true,
			Default:  models.DefaultValue(1)},
		"Discount": models.FloatField{
			String:  "Discount (%)",
			Digits:  decimalPrecision.GetPrecision("Discount"),
			Default: models.DefaultValue(0.0)},
		"InvoiceLineTaxes": models.Many2ManyField{
			String:        "Taxes",
			RelationModel: h.AccountTax(),
			JSON:          "invoice_line_tax_ids",
			Filter: q.AccountTax().TypeTaxUse().NotEquals("none").AndCond(
				q.AccountTax().Active().Equals(true).Or().Active().Equals(false))},
		"AccountAnalytic": models.Many2OneField{
			String:        "Analytic Account",
			RelationModel: h.AccountAnalyticAccount()},
		"AnalyticTags": models.Many2ManyField{
			RelationModel: h.AccountAnalyticTag(),
			JSON:          "analytic_tag_ids"},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Related:       "Invoice.Company",
			ReadOnly:      true},
		"Partner": models.Many2OneField{
			String:        "Partner",
			RelationModel: h.Partner(),
			Related:       "Invoice.Partner",
			ReadOnly:      true},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency(),
			Related:       "Invoice.Currency"},
		"CompanyCurrency": models.Many2OneField{
			RelationModel: h.Currency(),
			Related:       "Invoice.CompanyCurrency",
			ReadOnly:      true},
	})

	h.AccountInvoiceLine().Methods().GetAnalyticLine().DeclareMethod(
		`GetAnalyticLine`,
		func(rs m.AccountInvoiceLineSet) m.AccountAnalyticLineData {
			return h.AccountAnalyticLine().NewData().
				SetName(rs.Name()).
				SetDate(rs.Invoice().DateInvoice()).
				SetAccount(rs.AccountAnalytic()).
				SetUnitAmount(rs.Quantity()).
				SetAmount(rs.PriceSubtotalSigned()).
				SetProduct(rs.Product()).
				SetProductUom(rs.Uom()).
				SetGeneralAccount(rs.Account()).
				SetRef(rs.Invoice().Number())
		})

	h.AccountInvoiceLine().Methods().ComputePrice().DeclareMethod(
		`ComputePrice`,
		func(rs m.AccountInvoiceLineSet) m.AccountInvoiceLineData {
			currency := h.Currency().NewSet(rs.Env())
			if rs.Invoice().IsNotEmpty() {
				currency = rs.Invoice().Currency()
			}
			price := rs.PriceUnit() * (1 - rs.Discount()/100)
			var taxes float64
			if rs.InvoiceLineTaxes().IsNotEmpty() {
				_, taxes, _, _ = rs.InvoiceLineTaxes().ComputeAll(price, currency, rs.Quantity(), rs.Product(), rs.Invoice().Partner())
			}
			data := h.AccountInvoiceLine().NewData()
			priceSubtotalSigned := rs.Quantity() * price
			if taxes != 0.0 {
				priceSubtotalSigned = taxes
			}
			data.SetPriceSubtotal(priceSubtotalSigned)
			if rs.Invoice().Currency().IsNotEmpty() && rs.Invoice().Company().IsNotEmpty() && !rs.Invoice().Currency().Equals(rs.Invoice().Company().Currency()) {
				priceSubtotalSigned = rs.Invoice().Currency().WithContext("date", rs.Invoice().DateInvoice().ToDateTime()).Compute(priceSubtotalSigned, rs.Invoice().Company().Currency(), true)
			}
			sign := 1.0
			if strutils.IsIn(rs.Invoice().Type(), "in_refund", "out_refund") {
				sign = -1.0
			}
			data.SetPriceSubtotalSigned(priceSubtotalSigned * sign)
			return data
		})

	h.AccountInvoiceLine().Methods().FieldsViewGet().Extend("",
		func(rs m.AccountInvoiceLineSet, args webdata.FieldsViewGetParams) *webdata.FieldsViewData {
			res := rs.Super().FieldsViewGet(args)
			typ := rs.Env().Context().GetString("type")
			if typ == "" {
				return res
			}
			/*
				doc = etree.XML(res['arch'])    //tovalid what do?
				for node in doc.xpath("//field[@name='product_id']"):
					if self._context['type'] in ('in_invoice', 'in_refund'):
						# Hack to fix the stable version 8.0 -> saas-12
						# purchase_ok will be moved from purchase to product in master #13271
						if 'purchase_ok' in self.env['product.template']._fields:
							node.set('domain', "[('purchase_ok', '=', True)]")
					else:
						node.set('domain', "[('sale_ok', '=', True)]")
				res['arch'] = etree.tostring(doc)
			*/

			return res
		})

	h.AccountInvoiceLine().Methods().GetInvoiceLineAccount().DeclareMethod(
		`GetInvoiceLineAccount`,
		func(rs m.AccountInvoiceLineSet, typ string, product m.ProductProductSet, fPos m.AccountFiscalPositionSet,
			company m.CompanySet) m.AccountAccountSet {
			accountsInc, accountsExp := product.ProductTmpl().GetProductAccounts(fPos)
			if strutils.IsIn(typ, "out_income", "out_refund") {
				return accountsInc
			}
			return accountsExp
		})

	h.AccountInvoiceLine().Methods().DefineTaxes().DeclareMethod(
		`DefineTaxes is used in Onchange to set taxes and price.`,
		func(rs m.AccountInvoiceLineSet) {
			var taxes m.AccountTaxSet
			if strutils.IsIn(rs.Invoice().Type(), "out_invoice", "out_refund") {
				taxes = h.AccountTax().Coalesce(rs.Product().Taxes(), rs.Account().Taxes())
			} else {
				taxes = h.AccountTax().Coalesce(rs.Product().SupplierTaxes(), rs.Account().Taxes())
			}

			// Keep only taxes of the company
			company := h.Company().Coalesce(rs.Company(), h.User().NewSet(rs.Env()).CurrentUser().Company())
			taxes = taxes.Filtered(func(rs m.AccountTaxSet) bool { return rs.Company().Equals(company) })

			data := h.AccountInvoiceLine().NewData()
			fpTaxes := rs.Invoice().FiscalPosition().MapTax(taxes, rs.Product(), rs.Invoice().Partner())
			data.SetInvoiceLineTaxes(fpTaxes)
			fixPrice := h.AccountTax().NewSet(rs.Env()).FixTaxIncludedPrice
			if strutils.IsIn(rs.Invoice().Type(), "in_invoice", "in_refund") {
				prec := decimalPrecision.GetPrecision("Product Price").ToPrecision()
				if rs.PriceUnit() == 0.0 || nbutils.Compare(rs.PriceUnit(), rs.Product().StandardPrice(), prec) == 0 {
					data.SetPriceUnit(fixPrice(rs.Product().StandardPrice(), taxes, fpTaxes))
				}
			} else {
				data.SetPriceUnit(fixPrice(rs.Product().LstPrice(), taxes, fpTaxes))
			}
			rs.Write(data)
		})

	h.AccountInvoiceLine().Methods().OnchangeProduct().DeclareMethod(
		`OnchangeProduct`,
		func(rs m.AccountInvoiceLineSet) m.AccountInvoiceLineData {
			if rs.Invoice().IsEmpty() {
				return h.AccountInvoiceLine().NewData()
			}
			part := rs.Invoice().Partner()
			fpos := rs.Invoice().FiscalPosition()
			company := rs.Invoice().Company()
			currency := rs.Invoice().Currency()
			typ := rs.Invoice().Type()

			if part.IsEmpty() {
				/*
						  warning = {
					              'title': _('Warning!'),
					              'message': _('You must first select a partner!'),
					          }
					      return {'warning': warning}    //tovalid Warning field missing in AccountInvoiceLine
				*/
			}
			if rs.Product().IsEmpty() {
				if strutils.IsIn(typ, "in_invoice", "in_refund") {
					rs.SetPriceUnit(0.0)
				}
				/*
					domain['uom_id'] = []
					return {'domain': domain} //tovalid  domain field missing in AccountInvoiceLine
				*/
				return h.AccountInvoiceLine().NewData()
			}
			product := rs.Product()
			if part.Lang() != "" {
				product = product.WithContext("lang", part.Lang())
			}
			rs.SetName(product.PartnerRef())
			account := rs.GetInvoiceLineAccount(typ, product, fpos, company)
			if account.IsNotEmpty() {
				rs.SetAccount(account)
			}
			rs.DefineTaxes()

			if strutils.IsIn(typ, "in_invoice", "in_refund") && product.DescriptionPurchase() != "" {
				rs.SetName(rs.Name() + "\n" + product.DescriptionPurchase())
			} else if product.DescriptionSale() != "" {
				rs.SetName(rs.Name() + "\n" + product.DescriptionSale())
			}

			if rs.Uom().IsNotEmpty() || !product.Uom().Category().Equals(rs.Uom().Category()) {
				rs.SetUom(product.Uom())
			}
			if company.IsNotEmpty() && currency.IsNotEmpty() {
				if rs.Uom().IsNotEmpty() && !rs.Uom().Equals(product.Uom()) {
					rs.SetPriceUnit(product.Uom().ComputePrice(rs.PriceUnit(), rs.Uom()))
				}
			}
			/*
				domain['uom_id'] = [('category_id', '=', product.uom_id.category_id.id)]
				return {'domain': domain} //tovalid  domain field missing in AccountInvoiceLine
			*/
			return h.AccountInvoiceLine().NewData()
		})

	h.AccountInvoiceLine().Methods().OnchangeAccount().DeclareMethod(
		`OnchangeAccount`,
		func(rs m.AccountInvoiceLineSet) m.AccountInvoiceLineData {
			data := h.AccountInvoiceLine().NewData()
			if rs.Account().IsEmpty() {
				return data
			}
			if rs.Product().IsEmpty() {
				fpos := rs.Invoice().FiscalPosition()
				data.SetInvoiceLineTaxes(fpos.MapTax(rs.Account().Taxes(), h.ProductProduct().NewSet(rs.Env()), rs.Partner()))
			} else if rs.PriceUnit() == 0.0 {
				rs.DefineTaxes()
			}
			return data
		})

	h.AccountInvoiceLine().Methods().OnchangeUom().DeclareMethod(
		`OnchangeUom`,
		func(rs m.AccountInvoiceLineSet) m.AccountInvoiceLineData {
			data := h.AccountInvoiceLine().NewData()
			if rs.Uom().IsEmpty() {
				data.SetPriceUnit(0.0)
			} else if rs.Product().IsNotEmpty() && !rs.Product().Uom().Category().Equals(rs.Uom().Category()) {
				/*
						      warning = { //tovalid  warning field missing in AccountInvoiceLine
					              'title': _('Warning!'),
					              'message': _('The selected unit of measure is not compatible with the unit of measure of the product.'),
					          }
						      result['warning'] = warning
				*/
				data.SetUom(rs.Product().Uom())
			}
			return data
		})

	h.AccountInvoiceLine().Methods().DefineAdditionalFields().DeclareMethod(
		`DefineAdditionalFields
                  Some modules, such as Purchase, provide a feature to add automatically pre-filled
			      invoice lines. However, these modules might not be aware of extra fields which are
			      added by extensions of the accounting module.
			      This method is intended to be overridden by these extensions, so that any new field can
			      easily be auto-filled as well.
			      :param invoice : account.invoice corresponding record
			      :rtype line : account.invoice.line record`,
		func(rs m.AccountInvoiceLineSet, invoice m.AccountInvoiceSet) m.AccountInvoiceLineSet {
			return h.AccountInvoiceLine().NewSet(rs.Env())
		})

	h.AccountInvoiceLine().Methods().Unlink().Extend("",
		func(rs m.AccountInvoiceLineSet) int64 {
			if rs.Filtered(func(rs m.AccountInvoiceLineSet) bool {
				return rs.Invoice().IsNotEmpty() && rs.Invoice().State() != "draft"
			}).IsNotEmpty() {
				panic(rs.T(`You can only delete an invoice line if the invoice is in draft state.`))
			}
			return rs.Super().Unlink()
		})

	h.AccountInvoiceTax().DeclareModel()
	h.AccountInvoiceTax().SetDefaultOrder("Sequence")

	h.AccountInvoiceTax().AddFields(map[string]models.FieldDefinition{
		"Invoice": models.Many2OneField{
			RelationModel: h.AccountInvoice(),
			OnDelete:      models.Cascade,
			Index:         true},
		"Name": models.CharField{
			String:   "Tax Description",
			Required: true},
		"Tax": models.Many2OneField{
			RelationModel: h.AccountTax(),
			OnDelete:      models.Restrict},
		"Account": models.Many2OneField{
			String:        "Tax Account",
			RelationModel: h.AccountAccount(),
			Required:      true,
			Filter:        q.AccountAccount().Deprecated().Equals(false)},
		"AccountAnalytic": models.Many2OneField{
			String:        "Analytic account",
			RelationModel: h.AccountAnalyticAccount()},
		"Amount": models.FloatField{},
		"Manual": models.BooleanField{
			Default: models.DefaultValue(true)},
		"Sequence": models.IntegerField{
			Help: "Gives the sequence order when displaying a list of invoice tax."},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Related:       "Account.Company",
			ReadOnly:      true},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency(),
			Related:       "Invoice.Currency",
			ReadOnly:      true},
		"Base": models.FloatField{
			Compute: h.AccountInvoiceTax().Methods().ComputeBaseAmount()},
	})

	h.AccountInvoiceTax().Methods().ComputeBaseAmount().DeclareMethod(
		`ComputeBaseAmount`,
		func(rs m.AccountInvoiceTaxSet) m.AccountInvoiceTaxData {
			taxData := h.AccountInvoiceTax().NewData()
			taxData.SetBase(0.0)
			if rs.Tax().IsEmpty() {
				return taxData
			}
			key := rs.Tax().GetGroupingKey(h.AccountInvoiceTax().NewData().
				SetTax(rs.Tax()).
				SetAccount(rs.Account()).
				SetAccountAnalytic(rs.AccountAnalytic()))
			if AITdata, ok := rs.Invoice().GetTaxesValues()[key]; rs.Invoice().IsNotEmpty() && ok {
				taxData.SetBase(AITdata.Base())
			} else {
				log.Warn(`Tax Base Amount not computable probably due to a change in an underlying tax (%s).`, "tax", rs.Tax().Name())
			}
			return taxData
		})

	h.AccountPaymentTerm().DeclareModel()
	h.AccountPaymentTerm().SetDefaultOrder("Name")

	h.AccountPaymentTerm().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:    "Payment Terms",
			Translate: true,
			Required:  true},
		"Active": models.BooleanField{
			String:  "Active",
			Default: models.DefaultValue(true),
			Help:    "If the active field is set to False, it will allow you to hide the payment term without removing it."},
		"Note": models.TextField{
			String:    "Description on the Invoice",
			Translate: true},
		"Lines": models.One2ManyField{
			RelationModel: h.AccountPaymentTermLine(),
			ReverseFK:     "Payment",
			JSON:          "line_ids",
			String:        "Terms",
			//Default: func(env models.Environment) interface{} {
			//	return h.AccountPaymentTermLine().Create(env, h.AccountPaymentTermLine().NewData().
			//		SetValue("balance").
			//		SetValueAmount(0).
			//		SetSequence(9).
			//		SetDays(0).
			//		SetOption("day_after_invoice_date"))
			//},
			Constraint: h.AccountPaymentTerm().Methods().CheckLines()},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Required:      true,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser().Company()
			}},
	})

	h.AccountPaymentTerm().Methods().CheckLines().DeclareMethod(
		`CheckLines`,
		func(rs m.AccountPaymentTermSet) {
			paymentTermLines := rs.Lines().SortedDefault()
			PTLlen := paymentTermLines.Len()
			if PTLlen > 0 && paymentTermLines.Records()[PTLlen-1].Value() != "balance" {
				panic(rs.T(`A Payment Term should have its last line of type Balance.`))
			}
			PTLlen = rs.Lines().Filtered(func(rs m.AccountPaymentTermLineSet) bool { return rs.Value() == "balance" }).Len()
			if PTLlen > 1 {
				panic(rs.T(`A Payment Term should have only one line of type Balance.`))
			}
		})

	h.AccountPaymentTerm().Methods().Compute().DeclareMethod(
		`Compute`,
		func(rs m.AccountPaymentTermSet, value float64, dateRef dates.Date) []accounttypes.PaymentDueDates {
			var result []accounttypes.PaymentDueDates
			if dateRef.IsZero() {
				dateRef = dates.Today()
			}
			amount := value
			sign := 1.0
			if value < 0 {
				sign = -1.0
			}
			var currency m.CurrencySet
			if val := rs.Env().Context().GetInteger("currency_id"); val > 0 {
				currency = h.Currency().BrowseOne(rs.Env(), val)
			} else {
				currency = h.User().NewSet(rs.Env()).CurrentUser().Company().Currency()
			}
			prec := math.Pow10(int(-currency.DecimalPlaces()))
			for _, line := range rs.Lines().Records() {
				amt := 0.0
				switch line.Value() {
				case "fixed":
					amt = sign * nbutils.Round(line.ValueAmount(), prec)
				case "percent":
					amt = nbutils.Round(value*(line.ValueAmount()/100), prec)
				case "balance":
					amt = nbutils.Round(amount, prec)
				}
				if amt == 0.0 {
					continue
				}
				nextDate := dateRef
				switch line.Option() {
				case "day_after_invoice_date":
					nextDate = nextDate.AddDate(0, 0, int(line.Days()))
				case "fix_day_following_month":
					nextDate = nextDate.AddDate(0, 1, 1).AddDate(0, 0, int(line.Days())-1)
				case "last_day_following_month":
					nextDate = nextDate.AddDate(0, 1, 31)
				case "last_day_current_month":
					nextDate = nextDate.AddDate(0, 0, 31)
				}
				result = append(result, accounttypes.PaymentDueDates{Date: nextDate, Amount: amt})
				amount -= amt
			}
			amount = 0.0 //  reduce(lambda x, y: x + y[1], result, 0.0) //tovalid wat?
			dist := nbutils.Round(value-amount, prec)
			if dist != 0.0 {
				lastDate := dates.Today()
				if len := len(result); len > 0 {
					lastDate = result[len-1].Date
				}
				result = append(result, accounttypes.PaymentDueDates{Date: lastDate, Amount: dist})
			}
			return result
		})

	h.AccountPaymentTerm().Methods().Unlink().Extend("",
		func(rs m.AccountPaymentTermSet) int64 {
			//@api.multi
			/*def unlink(self):
			property_recs = self.env['ir.property'].search([('value_reference', 'in', ['account.payment.term,%s'%payment_term.id for payment_term in self])])
			property_recs.unlink() //tovalid h.irProperty ?
			return super(AccountPaymentTerm, self).unlink()
			*/
			return rs.Super().Unlink()
		})

	h.AccountPaymentTermLine().DeclareModel()
	h.AccountPaymentTermLine().SetDefaultOrder("Sequence", "ID")

	h.AccountPaymentTermLine().AddFields(map[string]models.FieldDefinition{
		"Value": models.SelectionField{
			Selection: types.Selection{
				"balance": "Balance",
				"percent": "Percent",
				"fixed":   "Fixed Amount"},
			String:     "Type",
			Required:   true,
			Default:    models.DefaultValue("balance"),
			Constraint: h.AccountPaymentTermLine().Methods().CheckPercent(),
			Help:       "Select here the kind of valuation related to this payment term line."},
		"ValueAmount": models.FloatField{
			String:     "Value",
			Digits:     decimalPrecision.GetPrecision("Payment Terms"),
			Constraint: h.AccountPaymentTermLine().Methods().CheckPercent(),
			Help:       "For percent enter a ratio between 0-100."},
		"Days": models.IntegerField{
			String:   "Number of Days",
			Required: true,
			Default:  models.DefaultValue(0)},
		"Option": models.SelectionField{
			String: "Options",
			Selection: types.Selection{
				"day_after_invoice_date":   "Day(s) after the invoice date",
				"fix_day_following_month":  "Day(s) after the end of the invoice month (Net EOM)",
				"last_day_following_month": "Last day of following month",
				"last_day_current_month":   "Last day of current month"},
			Default:  models.DefaultValue("day_after_invoice_date"),
			Required: true,
			OnChange: h.AccountPaymentTermLine().Methods().OnchangeOption()},
		"Payment": models.Many2OneField{
			String:        "Payment Terms",
			RelationModel: h.AccountPaymentTerm(),
			Required:      true,
			Index:         true,
			OnDelete:      models.Cascade},
		"Sequence": models.IntegerField{
			Default: models.DefaultValue(10),
			Help:    "Gives the sequence order when displaying a list of payment term lines."},
	})

	h.AccountPaymentTermLine().Methods().CheckPercent().DeclareMethod(
		`CheckPercent`,
		func(rs m.AccountPaymentTermLineSet) {
			if rs.Value() == "percent" && (rs.ValueAmount() < 0.0 || rs.ValueAmount() > 100) {
				panic(rs.T(`Percentages for Payment Terms Line must be between 0 and 100.`))
			}
		})

	h.AccountPaymentTermLine().Methods().OnchangeOption().DeclareMethod(
		`OnchangeOption`,
		func(rs m.AccountPaymentTermLineSet) m.AccountPaymentTermLineData {
			if strutils.IsIn(rs.Option(), "last_day_current_month", "last_day_following_month") {
				return h.AccountPaymentTermLine().NewData().SetDays(0)
			}
			return h.AccountPaymentTermLine().NewData()
		})

	//h.MailComposeMessage().Methods().SendMail().DeclareMethod(
	//	`SendMail`,
	//	func(rs h.MailComposeMessageSet, args struct {
	//		AutoCommit interface{}
	//	}) {
	//		//@api.multi
	//		/*def send_mail(self, auto_commit=False):
	//		  context = self._context
	//		  if context.get('default_model') == 'account.invoice' and \
	//		          context.get('default_res_id') and context.get('mark_invoice_as_sent'):
	//		      invoice = self.env['account.invoice'].browse(context['default_res_id'])
	//		      invoice = invoice.with_context(mail_post_autofollow=True)
	//		      invoice.sent = True
	//		      invoice.message_post(body=_("Invoice sent"))
	//		  return super(MailComposeMessage, self).send_mail(auto_commit=auto_commit)
	//		*/
	//	})

}
