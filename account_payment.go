// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"fmt"
	"math"
	"strings"

	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/strutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func init() {

	h.AccountPaymentMethod().DeclareModel()
	h.AccountPaymentMethod().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			Required:  true,
			Translate: true},
		"Code": models.CharField{
			Required: true},
		"PaymentType": models.SelectionField{
			Selection: types.Selection{
				"inbound":  "Inbound",
				"outbound": "Outbound"},
			Required: true},
	})

	h.AccountAbstractPayment().DeclareMixinModel()
	h.AccountAbstractPayment().AddFields(map[string]models.FieldDefinition{
		"PaymentType": models.SelectionField{
			Selection: types.Selection{
				"outbound": "Send Money",
				"inbound":  "Receive Money"},
			Required: true},
		"PaymentMethod": models.Many2OneField{
			String:        "Payment Method Type",
			RelationModel: h.AccountPaymentMethod(),
			Required:      true},
		"PaymentMethodCode": models.CharField{
			Help:     "Technical field used to adapt the interface to the payment type selected.",
			ReadOnly: true},
		"PartnerType": models.SelectionField{
			Selection: types.Selection{
				"customer": "Customer",
				"supplier": "Vendor"}},
		"Partner": models.Many2OneField{
			RelationModel: h.Partner()},
		"Amount": models.FloatField{
			String:     "Payment Amount",
			Required:   true,
			Constraint: h.AccountAbstractPayment().Methods().CheckAmount()},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency(),
			Required:      true,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser().Company().Currency()
			}},
		"PaymentDate": models.DateField{
			Default: func(env models.Environment) interface{} {
				return dates.Today()
			},
			Required: true,
			NoCopy:   true},
		"Communication": models.CharField{
			String: "Memo"},
		"Journal": models.Many2OneField{
			String:        "Payment Journal",
			RelationModel: h.AccountJournal(),
			Required:      true,
			Filter:        q.AccountJournal().Type().In([]string{"bank", "cash"}),
			OnChange:      h.AccountAbstractPayment().Methods().OnchangeJournal()},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Related:       "Journal.Company",
			ReadOnly:      true},
		"HidePaymentMethod": models.BooleanField{
			Compute: h.AccountAbstractPayment().Methods().ComputeHidePaymentMethod(),
			Help: `Technical field used to hide the payment method if the selected journal
has only one available which is 'manual'`},
	})

	h.AccountAbstractPayment().Methods().CheckAmount().DeclareMethod(
		`CheckAmount`,
		func(rs m.AccountAbstractPaymentSet) {
			if !(rs.Amount() > 0.0) {
				panic(rs.T(`The payment amount must be strictly positive.`))
			}
		})

	h.AccountAbstractPayment().Methods().ComputeHidePaymentMethod().DeclareMethod(
		`ComputeHidePaymentMethod`,
		func(rs m.AccountAbstractPaymentSet) m.AccountAbstractPaymentData {
			var data m.AccountAbstractPaymentData
			var journalPaymentMethods m.AccountPaymentMethodSet

			data = h.AccountAbstractPayment().NewData()
			if rs.Journal().IsEmpty() {
				data.SetHidePaymentMethod(true)
				return data
			}
			if rs.PartnerType() == "inbound" {
				journalPaymentMethods = rs.Journal().InboundPaymentMethods()
			} else {
				journalPaymentMethods = rs.Journal().OutboundPaymentMethods()
			}
			data.SetHidePaymentMethod(journalPaymentMethods.Len() == 1 && journalPaymentMethods.Code() == "manual")
			return data
		})

	h.AccountAbstractPayment().Methods().OnchangeJournal().DeclareMethod(
		`OnchangeJournal`,
		func(rs m.AccountAbstractPaymentSet) m.AccountAbstractPaymentData {
			var data m.AccountAbstractPaymentData
			var paymentMethods m.AccountPaymentMethodSet
			var paymentType string

			data = h.AccountAbstractPayment().NewData()
			if rs.Journal().IsEmpty() {
				return data
			}
			data.SetCurrency(h.Currency().Coalesce(rs.Journal().Currency(), rs.Company().Currency()))
			// Set default payment method (we consider the first to be the default one)
			if rs.PaymentType() == "inbound" {
				paymentMethods = rs.Journal().InboundPaymentMethods()
			} else {
				paymentMethods = rs.Journal().OutboundPaymentMethods()
			}
			data.SetPaymentMethod(paymentMethods.Records()[0])
			// Set payment method domain (restrict to methods enabled for the journal and to selected payment type)
			if strutils.IsIn(rs.PaymentType(), "outbound", "transfer") {
				paymentType = "outbound"
			} else {
				paymentType = "inbound"
			}

			//return {'domain': {'payment_method_id': [('payment_type', '=', payment_type), ('id', 'in', payment_methods.ids)]}}
			fmt.Println(paymentType)
			return data
		})

	h.AccountAbstractPayment().Methods().GetInvoices().DeclareMethod(
		`GetInvoices Return the invoices of the payment. Must be overridden `,
		func(rs m.AccountAbstractPaymentSet) m.AccountInvoiceSet {
			panic(rs.T("Not implemented"))
		})

	h.AccountAbstractPayment().Methods().ComputeTotalInvoicesAmount().DeclareMethod(
		`ComputeTotalInvoicesAmount Compute the sum of the residual of invoices, expressed in the payment currency`,
		func(rs m.AccountAbstractPaymentSet) float64 {
			var paymentCurrency m.CurrencySet
			var invoices m.AccountInvoiceSet
			var all bool
			var total float64

			paymentCurrency = h.Currency().Coalesce(
				rs.Currency(),
				rs.Journal().Currency(),
				rs.Journal().Company().Currency(),
				h.User().NewSet(rs.Env()).CurrentUser().Company().Currency())
			invoices = rs.GetInvoices()

			all = true
			for _, inv := range invoices.Records() {
				if !(inv.Currency().Equals(paymentCurrency)) {
					all = false
					break
				}
			}

			if all {
				for _, inv := range invoices.Records() {
					total += inv.ResidualSigned()
				}
				return math.Abs(total)
			}
			//else
			for _, inv := range invoices.Records() {
				if inv.CompanyCurrency().Equals(paymentCurrency) {
					total += inv.ResidualCompanySigned()
				} else {
					total += inv.CompanyCurrency().WithContext("date", rs.PaymentDate()).Compute(inv.ResidualCompanySigned(), paymentCurrency, true)
				}
			}
			return math.Abs(total)
		})

	h.AccountRegisterPayments().DeclareTransientModel()
	h.AccountRegisterPayments().InheritModel(h.AccountAbstractPayment())

	// h.AccountRegisterPayments().Fields().PaymentType().SetOnchange(h.AccountRegisterPayments().Methods().OnchangePaymentType())

	h.AccountRegisterPayments().Methods().OnchangePaymentType().DeclareMethod(
		`OnchangePaymentType`,
		func(rs m.AccountRegisterPaymentsSet) m.AccountRegisterPaymentsData {
			var data m.AccountRegisterPaymentsData

			data = h.AccountRegisterPayments().NewData()
			if rs.PaymentType() != "" {
				/*data = {'domain': {'payment_method_id': [('payment_type', '=', self.payment_type)]}} tovalid*/
			}
			return data
		})

	h.AccountRegisterPayments().Methods().GetInvoices().Extend(
		"Return the invoices of the payment. Must be overridden",
		func(rs m.AccountRegisterPaymentsSet) m.AccountInvoiceSet {
			if rs.Env().Context().HasKey("active_ids") {
				return h.AccountInvoice().Browse(rs.Env(), rs.Env().Context().GetIntegerSlice("active_ids"))
			}
			return h.AccountInvoice().NewSet(rs.Env())
		})

	h.AccountRegisterPayments().Methods().DefaultGet().Extend("",
		func(rs m.AccountRegisterPaymentsSet) m.AccountRegisterPaymentsData {
			rec := rs.Super().DefaultGet()
			context := rs.Env().Context()
			activeModel := context.GetString("active_model")
			activeIds := context.GetIntegerSlice("active_ids")
			if activeModel == "" || len(activeIds) == 0 {
				panic(rs.T(`Programing error: wizard action executed without active_model or active_ids in context.`))
			}
			if !strutils.IsIn(activeModel, "account.invoice", "AccountInvoice") {
				panic(rs.T(`Programing error: the expected model for this action is 'account.invoice'. The provided one is '%d'.`, activeModel))
			}

			// Checks on received invoice records
			var (
				communication string
				totalAmount   float64
			)
			invoices := h.AccountInvoice().Browse(rs.Env(), activeIds)
			for _, inv := range invoices.Records() {
				switch {
				case inv.State() != "open":
					panic(rs.T(`You can only register payments for open invoices`))
				case !inv.CommercialPartner().Equals(invoices.CommercialPartner()):
					panic(rs.T(`In order to pay multiple invoices at once, they must belong to the same commercial partner.`))
				case Type2PartnerType[inv.Type()] != Type2PartnerType[invoices.Type()]:
					panic(rs.T(`You cannot mix customer invoices and vendor bills in a single payment.`))
				case !inv.Currency().Equals(invoices.Currency()):
					panic(rs.T(`In order to pay multiple invoices at once, they must use the same currency.`))
				}
				totalAmount += inv.Residual() * Type2PaymentType[inv.Type()]
				if inv.Reference() != "" {
					communication = communication + " " + inv.Reference()
				}
			}
			communication = strings.TrimPrefix(communication, " ")

			rec.SetAmount(math.Abs(totalAmount))
			rec.SetCurrency(invoices.Currency())
			rec.SetPaymentType("outbound")
			if totalAmount > 0 {
				rec.SetPaymentType("inbound")
			}
			rec.SetPartner(invoices.CommercialPartner())
			rec.SetPartnerType(Type2PartnerType[invoices.Type()])
			rec.SetCommunication(communication)
			return rec
		})

	h.AccountRegisterPayments().Methods().GetPaymentVals().DeclareMethod(
		`GetPaymentVals Hook for extension `,
		func(rs m.AccountRegisterPaymentsSet) m.AccountPaymentData {
			return h.AccountPayment().NewData().
				SetJournal(rs.Journal()).
				SetPaymentMethod(rs.PaymentMethod()).
				SetPaymentDate(rs.PaymentDate()).
				SetCommunication(rs.Communication()).
				SetInvoices(rs.GetInvoices()).
				SetPaymentType(rs.PaymentType()).
				SetAmount(rs.Amount()).
				SetCurrency(rs.Currency()).
				SetPartner(rs.Partner()).
				SetPartnerType(rs.PartnerType())
		})

	h.AccountRegisterPayments().Methods().CreatePayment().DeclareMethod(
		`CreatePayment`,
		func(rs m.AccountRegisterPaymentsSet) *actions.Action {
			payment := h.AccountPayment().Create(rs.Env(), rs.GetPaymentVals())
			payment.Post()
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

	h.AccountPayment().DeclareModel()
	h.AccountPayment().InheritModel(h.AccountAbstractPayment())
	h.AccountPayment().SetDefaultOrder("PaymentDate DESC", "Name DESC")

	h.AccountPayment().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:   "Name",
			ReadOnly: true,
			NoCopy:   true,
			Default:  models.DefaultValue("Draft Payment")},
		"State": models.SelectionField{
			String: "Status",
			Selection: types.Selection{
				"draft":      "Draft",
				"posted":     "Posted",
				"sent":       "Sent",
				"reconciled": "Reconciled"},
			ReadOnly: true,
			Default:  models.DefaultValue("draft"),
			NoCopy:   true},
		"PaymentReference": models.CharField{
			String:   "PaymentReference",
			NoCopy:   true,
			ReadOnly: true,
			Help:     "Reference of the document used to issue this payment. Eg. check number, file name, etc."},
		"MoveName": models.CharField{
			String:   "Journal Entry Name",
			ReadOnly: true,
			Default:  models.DefaultValue(false),
			NoCopy:   true,
			Help: `Technical field holding the number given to the journal entry, automatically set when the statement
line is reconciled then stored to set the same number again if the line is cancelled,
set to draft and re-processed again." `},
		"DestinationAccount": models.Many2OneField{
			RelationModel: h.AccountAccount(),
			Compute:       h.AccountPayment().Methods().ComputeDestinationAccount(),
			Depends:       []string{"Invoices", "PaymentType", "PartnerType", "Partner"},
		},
		"DestinationJournal": models.Many2OneField{
			String:        "Transfer To",
			RelationModel: h.AccountJournal(),
			Filter:        q.AccountJournal().Type().In([]string{"bank", "cash"})},
		"Invoices": models.Many2ManyField{
			RelationModel: h.AccountInvoice(),
			JSON:          "invoice_ids",
			NoCopy:        true,
			ReadOnly:      true,
			OnChange:      h.AccountPayment().Methods().ComputeDestinationAccount()},
		"HasInvoice": models.BooleanField{
			Compute: h.AccountPayment().Methods().ComputeHasInvoice(),
			Help:    "Technical field used for usability purposes"},
		"PaymentDifference": models.FloatField{
			Compute: h.AccountPayment().Methods().ComputePaymentDifference()},
		"PaymentDifferenceHandling": models.SelectionField{
			String: "Payment Difference",
			Selection: types.Selection{
				"open":      "Keep open",
				"reconcile": "Mark invoice as fully paid"},
			Default: models.DefaultValue("open"),
			NoCopy:  true},
		"WriteoffAccount": models.Many2OneField{
			String:        "Difference Account",
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().Deprecated().Equals(false)},
		"MoveLines": models.One2ManyField{
			RelationModel: h.AccountMoveLine(),
			ReverseFK:     "Payment",
			JSON:          "move_line_ids",
			ReadOnly:      true},
	})

	h.AccountPayment().Fields().PaymentType().
		UpdateSelection(types.Selection{"transfer": "Internal Transfer"}).
		SetOnchange(h.AccountPayment().Methods().OnchangePaymentType())
	h.AccountPayment().Fields().PartnerType().
		SetOnchange(h.AccountPayment().Methods().OnchangePartnerType())

	h.AccountPayment().Methods().ComputeHasInvoice().DeclareMethod(
		`ComputeHasInvoice`,
		func(rs m.AccountPaymentSet) m.AccountPaymentData {
			data := h.AccountPayment().NewData().SetHasInvoice(rs.Invoices().IsNotEmpty())
			return data
		})

	h.AccountPayment().Methods().ComputePaymentDifference().DeclareMethod(
		`ComputePaymentDifference`,
		func(rs m.AccountPaymentSet) m.AccountPaymentData {
			var data m.AccountPaymentData

			data = h.AccountPayment().NewData()
			if rs.Invoices().IsEmpty() {
				return data
			}
			if strutils.IsIn(rs.Invoices().Type(), "in_invoice", "out_refund") {
				data.SetPaymentDifference(rs.Amount() - rs.ComputeTotalInvoicesAmount())
			} else {
				data.SetPaymentDifference(rs.ComputeTotalInvoicesAmount() - rs.Amount())
			}
			return data

		})

	h.AccountPayment().Methods().ComputeDestinationAccount().DeclareMethod(
		`ComputeDestinationAccountId`,
		func(rs m.AccountPaymentSet) m.AccountPaymentData {
			data := h.AccountPayment().NewData()
			switch {
			case rs.Invoices().IsNotEmpty():
				data.SetDestinationAccount(rs.Invoices().Account())
			case rs.PaymentType() == "transfer":
				if rs.Company().TransferAccount().IsEmpty() {
					panic(rs.T(`Transfer account not defined on the company.`))
				}
				data.SetDestinationAccount(rs.Company().TransferAccount())
			case rs.Partner().IsNotEmpty():
				if rs.PartnerType() == "customer" {
					data.SetDestinationAccount(rs.Partner().PropertyAccountReceivable())
				} else {
					data.SetDestinationAccount(rs.Partner().PropertyAccountPayable())
				}
			}
			return data
		})

	h.AccountPayment().Methods().OnchangePartnerType().DeclareMethod(
		`OnchangePartnerType`,
		func(rs m.AccountPaymentSet) m.AccountPaymentData {
			var data m.AccountPaymentData

			data = h.AccountPayment().NewData()
			// Set partner_id domain
			if rs.PartnerType() != "" {
				/* data = {'domain': {'partner_id': [(self.partner_type, '=', True)]}} tovalid */
			}
			return data
		})

	h.AccountPayment().Methods().OnchangePaymentType().DeclareMethod(
		`OnchangePaymentType`,
		func(rs m.AccountPaymentSet) m.AccountPaymentData {
			var data m.AccountPaymentData

			data = h.AccountPayment().NewData()
			// Set partner_id domain
			if rs.PartnerType() != "" {
				/* data = {'domain': {'partner_id': [(self.partner_type, '=', True)]}} tovalid */
			}
			return data
		})

	h.AccountPayment().Methods().DefaultGet().Extend("",
		func(rs m.AccountPaymentSet) m.AccountPaymentData {
			rec := rs.Super().DefaultGet()
			invoices := rs.Invoices()
			if invoices.IsEmpty() {
				return rec
			}
			invoice := invoices.Records()[0]
			val := invoice.Reference()
			if val == "" {
				val = invoice.Name()
			}
			if val == "" {
				val = invoice.Number()
			}
			rec.SetCommunication(val)
			rec.SetCurrency(invoice.Currency())
			if strutils.IsIn(invoice.Type(), "out_invoice", "in_refund") {
				rec.SetPaymentType("inbound")
			} else {
				rec.SetPaymentType("outbound")
			}
			rec.SetPartnerType(Type2PartnerType[invoice.Type()])
			rec.SetPartner(invoice.Partner())
			rec.SetAmount(invoice.Residual())
			return rec
		})

	h.AccountPayment().Methods().GetInvoices().Extend("",
		func(rs m.AccountPaymentSet) m.AccountInvoiceSet {
			return rs.Invoices()
		})

	h.AccountPayment().Methods().ButtonJournalEntries().DeclareMethod(
		`ButtonJournalEntries`,
		func(rs m.AccountPaymentSet) *actions.Action {
			return &actions.Action{
				Name:     rs.T(`Journal Items"`),
				Type:     actions.ActionActWindow,
				Model:    "AccountMoveLine",
				ViewMode: "tree,form",
				Domain:   "[('payment_id', 'in', rs.ids)]",
			}
		})

	h.AccountPayment().Methods().ButtonInvoices().DeclareMethod(
		`ButtonInvoices`,
		func(rs m.AccountPaymentSet) *actions.Action {
			return &actions.Action{
				Name:     rs.T(`Paid invoices`),
				Type:     actions.ActionActWindow,
				Model:    "AccountInvoice",
				ViewMode: "tree,form",
				Domain:   "[('id', 'in', [x.id for x in self.invoice_ids])]",
			}
		})

	h.AccountPayment().Methods().ButtonDummy().DeclareMethod(
		`ButtonDummy`,
		func(rs m.AccountPaymentSet) bool {
			return true
		})

	h.AccountPayment().Methods().Unreconcile().DeclareMethod(
		`Unreconcile Set back the payments in 'posted' or 'sent' state, without deleting the journal entries.
			      Called when cancelling a bank statement line linked to a pre-registered payment.`,
		func(rs m.AccountPaymentSet) {
			data := h.AccountPayment().NewData()
			for _, payment := range rs.Records() {
				if payment.PaymentReference() != "" {
					data.SetState("sent")
				} else {
					data.SetState("posted")
				}
				payment.Write(data)
			}
		})

	h.AccountPayment().Methods().Cancel().DeclareMethod(
		`Cancel`,
		func(rs m.AccountPaymentSet) {
			for _, rec := range rs.Records() {
				for _, moves := range rec.MoveLines().Records() {
					move := moves.Move()
					if rec.Invoices().IsNotEmpty() {
						move.Lines().RemoveMoveReconcile()
					}
					move.ButtonCancel()
					move.Unlink()
				}
				rec.SetState("draft")
			}
		})

	h.AccountPayment().Methods().Unlink().Extend("",
		func(rs m.AccountPaymentSet) int64 {
			for _, rec := range rs.Records() {
				if rec.MoveLines().IsNotEmpty() {
					panic(rs.T(`You can not delete a payment that is already posted`))
				}
				if rec.MoveName() != "" {
					panic(rs.T(`It is not allowed to delete a payment that already created a journal entry since it would create a gap in the numbering. You should create the journal entry again and cancel it thanks to a regular revert.`))
				}
			}
			return rs.Super().Unlink()
		})

	h.AccountPayment().Methods().Post().DeclareMethod(
		`Post Create the journal items for the payment and update the payment's state to 'posted'.
			      A journal entry is created containing an item in the source liquidity account (selected journal's default_debit or default_credit)
			      and another in the destination reconciliable account (see _compute_destination_account_id).
			      If invoice_ids is not empty, there will be one reconciliable move line per invoice to reconcile with.
			      If the payment is a transfer, a second journal entry is created in the destination journal to receive money from the transfer account.`,
		func(rs m.AccountPaymentSet) {

			for _, rec := range rs.Records() {
				if rec.State() != "draft" {
					panic(rs.T(`Only a draft payment can be posted. Trying to post a payment in state %s.`, rec.State()))
				}
				for _, inv := range rec.Invoices().Records() {
					if inv.State() != "open" {
						panic(rs.T(`The payment cannot be processed because the invoice is not open!`))
					}
				}

				data := h.AccountPayment().NewData()
				var sequenceCode string
				// Use the right sequence to set the name
				switch {
				case rec.PaymentType() == "transfer":
					sequenceCode = "account.payment.transfer"
				case rec.PaymentType() == "inbound" && rec.PartnerType() == "customer":
					sequenceCode = "account.payment.customer.invoice"
				case rec.PaymentType() == "inbound" && rec.PartnerType() == "supplier":
					sequenceCode = "account.payment.supplier.refund"
				case rec.PaymentType() == "outbound" && rec.PartnerType() == "customer":
					sequenceCode = "account.payment.customer.refund"
				case rec.PaymentType() == "outbound" && rec.PartnerType() == "supplier":
					sequenceCode = "account.payment.supplier.invoice"
				}
				name := h.Sequence().NewSet(rs.Env()).WithContext("ir_sequence_date", rec.PaymentDate()).NextByCode(sequenceCode)
				if name == "" && rec.PaymentType() != "transfer" {
					panic(rs.T("You have to define a sequence for %s in your company.", sequenceCode))
				}
				rec.SetName(name)

				// Create the journal entry
				sign := -1.0
				if strutils.IsIn(rec.PaymentType(), "outbound", "transfer") {
					sign = 1.0
				}
				amount := rec.Amount() * sign
				move := rec.CreatePaymentEntry(amount)

				// In case of a transfer, the first journal entry created debited the source liquidity account and credited
				// the transfer account. Now we debit the transfer account and credit the destination liquidity account.
				if rec.PaymentType() == "transfer" {
					transferCreditAml := move.Lines().Filtered(func(r m.AccountMoveLineSet) bool { return r.Account().Equals(rec.Company().TransferAccount()) })
					transferDebitAml := rec.CreateTransferEntry(amount)
					transferCreditAml.Union(transferDebitAml).Reconcile(h.AccountAccount().NewSet(rs.Env()), h.AccountJournal().NewSet(rs.Env()))
				}

				data.SetState("posted").
					SetMoveName(move.Name())

				rec.Write(data)
			}
		})

	h.AccountPayment().Methods().CreatePaymentEntry().DeclareMethod(
		`Create a journal entry corresponding to a payment, if the payment references invoice(s) they are reconciled.
			      Return the journal entry.`,
		func(rs m.AccountPaymentSet, amount float64) m.AccountMoveSet {
			env := rs.Env()
			amlObj := h.AccountMoveLine().NewSet(env).WithContext("check_move_validity", false)
			invoiceCurrency := h.Currency().NewSet(env)
			if rs.Invoices().IsNotEmpty() {
				targetCur := rs.Invoices().Records()[0].Currency()
				all := true
				for _, x := range rs.Invoices().Records() {
					if !x.Currency().Equals(targetCur) {
						all = false
						break
					}
				}
				if all {
					invoiceCurrency = targetCur
				}
			}
			debit, credit, amountCurrency, currency := amlObj.WithContext("date", rs.PaymentDate()).ComputeAmountFields(amount, rs.Currency(), rs.Company().Currency(), invoiceCurrency)

			move := h.AccountMove().Create(env, rs.GetMoveVals(h.AccountJournal().NewSet(env)))

			// Write line corresponding to invoice payment
			counterpartAmlDict := rs.GetSharedMoveLineVals(debit, credit, amountCurrency, move, h.AccountInvoice().NewSet(env))
			counterpartAmlDict.SetCurrency(currency)
			undrlyng := counterpartAmlDict.Underlying()
			undrlyng.MergeWith(rs.GetCounterpartMoveLineVals(rs.Invoices()).Underlying().FieldMap, h.AccountMoveLine().Model)
			counterpartAml := amlObj.Create(counterpartAmlDict)

			// Reconcile with the invoices
			if rs.PaymentDifferenceHandling() == "reconcile" && rs.PaymentDifference() != 0.0 {
				writeoffLine := rs.GetSharedMoveLineVals(0, 0, 0, move, h.AccountInvoice().NewSet(env))
				_, _, amountCurrencyWo, currency := amlObj.WithContext("date", rs.PaymentDate()).ComputeAmountFields(rs.PaymentDifference(), rs.Currency(), rs.Company().Currency(), invoiceCurrency)
				// the writeoff debit and credit must be computed from the invoice residual in company currency
				// minus the payment amount in company currency, and not from the payment difference in the payment currency
				// to avoid loss of precision during the currency rate computations. See revision 20935462a0cabeb45480ce70114ff2f4e91eaf79 for a detailed example.
				totalResidualCompanySigned := 0.0
				for _, invoice := range rs.Invoices().Records() {
					totalResidualCompanySigned += invoice.ResidualCompanySigned()
				}
				totalPaymentCompanySigned := rs.Currency().WithContext("date", rs.PaymentDate()).Compute(rs.Amount(), rs.Company().Currency(), true)
				amountWo := totalResidualCompanySigned - totalPaymentCompanySigned
				if strutils.IsIn(rs.Invoices().Records()[0].Type(), "in_invoice", "out_refund") {
					amountWo = totalPaymentCompanySigned - totalResidualCompanySigned
				}
				// Align the sign of the secondary currency writeoff amount with the sign of the writeoff
				// amount in the company currency
				debitWo := 0.0
				creditWo := -amountWo
				amountCurrencyWo = -math.Abs(amountCurrencyWo)
				if amountWo > 0 {
					debitWo = amountWo
					creditWo = 0.0
					amountCurrencyWo = math.Abs(amountCurrencyWo)
				}
				writeoffLine.
					SetName(rs.T("Counterpart")).
					SetAccount(rs.WriteoffAccount()).
					SetDebit(debitWo).
					SetCredit(creditWo).
					SetAmountCurrency(amountCurrencyWo).
					SetCurrency(currency)
				amlObj.Create(writeoffLine)
				if counterpartAml.Debit() != 0.0 {
					counterpartAml.SetDebit(counterpartAml.Debit() + (creditWo - debitWo))
				}
				if counterpartAml.Credit() != 0.0 {
					counterpartAml.SetCredit(counterpartAml.Credit() + (debitWo - creditWo))
				}
				counterpartAml.SetAmountCurrency(counterpartAml.AmountCurrency() - amountCurrencyWo)
			}
			rs.Invoices().WithContext("check_move_validity", false).RegisterPayment(counterpartAml, h.AccountAccount().NewSet(env), h.AccountJournal().NewSet(env))

			// Write counterpart lines
			if rs.Currency().Equals(rs.Company().Currency()) {
				amountCurrency = 0
			}
			liquidityAmlDict := rs.GetSharedMoveLineVals(credit, debit, -amountCurrency, move, h.AccountInvoice().NewSet(env))
			liquidityAmlDict.Underlying().MergeWith(rs.GetLiquidityMoveLineVals(-amount).Underlying().FieldMap, h.AccountMoveLine().Model)
			amlObj.Create(liquidityAmlDict)
			move.Post()
			return move
		})

	h.AccountPayment().Methods().CreateTransferEntry().DeclareMethod(
		`CreateTransferEntry Create the journal entry corresponding to the 'incoming money' part of an internal transfer, return the reconciliable move line`,
		func(rs m.AccountPaymentSet, amount float64) m.AccountMoveLineSet {

			amlObj := h.AccountMoveLine().NewSet(rs.Env()).WithContext("check_move_validity", false)
			debit, credit, _, _ := amlObj.WithContext("date", rs.PaymentDate()).ComputeAmountFields(amount, rs.Currency(), rs.Company().Currency(), h.Currency().NewSet(rs.Env()))
			var amountCurrency float64
			if rs.DestinationJournal().Currency().IsNotEmpty() {
				amountCurrency = rs.Currency().WithContext("date", rs.PaymentDate()).Compute(amount, rs.DestinationJournal().Currency(), true)
			}

			dstMove := h.AccountMove().Create(rs.Env(), rs.GetMoveVals(rs.DestinationJournal()))
			dstLiquidityAMLData := rs.GetSharedMoveLineVals(debit, credit, amountCurrency, dstMove, h.AccountInvoice().NewSet(rs.Env())).
				SetName(rs.T(`Transfer from %s`, rs.Journal().Name())).
				SetAccount(rs.DestinationJournal().DefaultCreditAccount()).
				SetCurrency(rs.DestinationJournal().Currency()).
				SetPayment(rs).
				SetJournal(rs.DestinationJournal())
			amlObj.Create(dstLiquidityAMLData)

			transferDebitAmlData := rs.GetSharedMoveLineVals(credit, debit, 0, dstMove, h.AccountInvoice().NewSet(rs.Env())).
				SetName(rs.Name()).
				SetPayment(rs).
				SetAccount(rs.Company().TransferAccount()).
				SetJournal(rs.DestinationJournal())
			if !rs.Currency().Equals(rs.Company().Currency()) {
				transferDebitAmlData.
					SetCurrency(rs.Currency()).
					SetAmountCurrency(-rs.Amount())
			}
			transferDebitAml := amlObj.Create(transferDebitAmlData)
			dstMove.Post()
			return transferDebitAml
		})

	h.AccountPayment().Methods().GetMoveVals().DeclareMethod(
		`GetMoveVals Return dict to create the payment move`,
		func(rs m.AccountPaymentSet, journal m.AccountJournalSet) m.AccountMoveData {
			var name string

			journal = h.AccountJournal().Coalesce(journal, rs.Journal())
			if journal.EntrySequence().IsEmpty() {
				panic(rs.T(`Configuration Error ! The journal %s does not have a sequence, please specify one.`, journal.Name()))
			} else if !journal.EntrySequence().Active() {
				panic(rs.T(`Configuration Error ! The sequence of journal %s is deactivated.`, journal.Name()))
			}
			name = rs.MoveName()
			if name == "" {
				name = journal.WithContext("ir_sequence_date", rs.PaymentDate()).EntrySequence().NextByID()
			}
			return h.AccountMove().NewData().
				SetName(name).
				SetDate(rs.PaymentDate()).
				SetRef(rs.Communication()).
				SetJournal(journal)
		})

	h.AccountPayment().Methods().GetSharedMoveLineVals().DeclareMethod(
		`GetSharedMoveLineVals Returns values common to both move lines (except for debit, credit and amount_currency which are reversed)`,
		func(rs m.AccountPaymentSet, debit, credit, amountCurrency float64, move m.AccountMoveSet,
			invoice m.AccountInvoiceSet) m.AccountMoveLineData {

			var data m.AccountMoveLineData

			data = h.AccountMoveLine().NewData().
				SetInvoice(invoice).
				SetMove(move).
				SetDebit(debit).
				SetCredit(credit).
				SetAmountCurrency(amountCurrency)
			if strutils.IsIn(rs.PaymentType(), "inbound", "outbound") {
				data.SetPartner(h.Partner().NewSet(rs.Env()).FindAccountingPartner(rs.Partner()))
			}
			return data
		})

	h.AccountPayment().Methods().GetCounterpartMoveLineVals().DeclareMethod(
		`GetCounterpartMoveLineVals`,
		func(rs m.AccountPaymentSet, invoice m.AccountInvoiceSet) m.AccountMoveLineData {
			var name string
			var CurrencyVal m.CurrencySet

			if rs.PaymentType() == "transfer" {
				name = rs.Name()
			} else {
				switch {
				case rs.PartnerType() == "customer" && rs.PaymentType() == "inbound":
					name = rs.T("Customer Payment")
				case rs.PartnerType() == "customer" && rs.PaymentType() == "outbound":
					name = rs.T("Customer Refund")
				case rs.PartnerType() == "supplier" && rs.PaymentType() == "inbound":
					name = rs.T("Vendor Refund")
				case rs.PartnerType() == "supplier" && rs.PaymentType() == "outbound":
					name = rs.T("Vendor Payment")
				}
				if invoice.IsNotEmpty() {
					name += ": "
					for _, inv := range invoice.Records() {
						if inv.Move().IsNotEmpty() {
							name += inv.Number() + ", "
						}
					}
					name = name[:len(name)-2]
				}
			}

			if !rs.Currency().Equals(rs.Company().Currency()) {
				CurrencyVal = rs.Currency()
			}
			res := h.AccountMoveLine().NewData().
				SetName(name).
				SetJournal(rs.Journal()).
				SetCurrency(CurrencyVal).
				SetPayment(rs).
				SetAccount(rs.DestinationAccount())
			return res
		})

	h.AccountPayment().Methods().GetLiquidityMoveLineVals().DeclareMethod(
		`GetLiquidityMoveLineVals`,
		func(rs m.AccountPaymentSet, amount float64) m.AccountMoveLineData {
			var name string
			var vals m.AccountMoveLineData
			name = rs.Name()
			if rs.PaymentType() == "transfer" {
				name = rs.T(`Transfer to %s`, rs.DestinationJournal().Name())
			}
			account := rs.Journal().DefaultCreditAccount()
			if strutils.IsIn(rs.PaymentType(), "outbound", "transfer") {
				account = rs.Journal().DefaultDebitAccount()
			}
			vals = h.AccountMoveLine().NewData().
				SetName(name).
				SetAccount(account).
				SetPayment(rs).
				SetJournal(rs.Journal())
			if !rs.Currency().Equals(rs.Company().Currency()) {
				vals.SetCurrency(rs.Currency())
			}

			// If the journal has a currency specified, the journal item need to be expressed in this currency
			if rs.Journal().Currency().IsNotEmpty() && !rs.Currency().Equals(rs.Journal().Currency()) {
				amount = rs.Currency().WithContext("date", rs.PaymentDate()).Compute(amount, rs.Journal().Currency(), true)
				_, _, amount, _ = h.AccountMoveLine().NewSet(rs.Env()).WithContext("date", rs.PaymentDate()).ComputeAmountFields(amount, rs.Journal().Currency(), rs.Company().Currency(), h.Currency().NewSet(rs.Env()))
				vals.SetAmountCurrency(amount).
					SetCurrency(rs.Journal().Currency())
			}

			return vals
		})

}
