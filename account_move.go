// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/hexya-addons/account/accounttypes"
	"github.com/hexya-addons/decimalPrecision"
	"github.com/hexya-addons/web/webdata"
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
	"github.com/hexya-erp/hexya/src/tools/strutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	"github.com/jmoiron/sqlx"
)

func init() {

	h.AccountMove().DeclareModel()
	h.AccountMove().SetDefaultOrder("Date DESC", "ID DESC")

	h.AccountMove().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:   "Number",
			Required: true,
			NoCopy:   true,
			Default:  models.DefaultValue("/")},
		"Ref": models.CharField{
			String: "Reference",
			NoCopy: true},
		"Date": models.DateField{
			String:   "Date",
			Required: true, /*[ states {'posted': [('readonly']*/ /*[ True)]}]*/
			Index:    true,
			Default:  models.DefaultValue(dates.Today())},
		"Journal": models.Many2OneField{RelationModel: h.AccountJournal(),
			Required: true, /*[ states {'posted': [('readonly']*/ /*[ True)]}]*/
			Default: func(env models.Environment) interface{} {
				if env.Context().HasKey("default_journal_type") {
					return h.AccountJournal().Search(env,
						q.AccountJournal().Type().Equals(env.Context().GetString("default_journal_type"))).Limit(1)
				}
				return h.AccountJournal().NewSet(env)
			}},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency(),
			Compute:       h.AccountMove().Methods().ComputeCurrency(),
			Stored:        true,
			Depends:       []string{"Company"}},
		"State": models.SelectionField{
			String: "Status",
			Selection: types.Selection{
				"draft":  "Unposted",
				"posted": "Posted"},
			Required: true,
			ReadOnly: true,
			NoCopy:   true,
			Default:  models.DefaultValue("draft"),
			Help: `All manually created new journal entries are usually in the status 'Unposted' but you can set the option
to skip that status on the related journal. In that case they will behave as journal entries
automatically created by the system on document validation (invoices, bank statements...) and
will be created in 'Posted' status.'`},
		"Lines": models.One2ManyField{
			String:        "Journal Items",
			RelationModel: h.AccountMoveLine(),
			ReverseFK:     "Move",
			JSON:          "line_ids", /*[ states {'posted': [('readonly']*/ /*[ True)]}]*/
			Copy:          true},
		"Partner": models.Many2OneField{
			RelationModel: h.Partner(),
			Compute:       h.AccountMove().Methods().ComputePartner(),
			Depends:       []string{"Lines", "Lines.Partner"},
			Stored:        true},
		"Amount": models.FloatField{
			Compute: h.AccountMove().Methods().AmountCompute(),
			Depends: []string{"Lines", "Lines.Debit", "Lines.Credit"},
			Stored:  true},
		"Narration": models.TextField{
			String: "Internal Note"},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Related:       "Journal.Company",
			ReadOnly:      true,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser().Company()
			}},
		"MatchedPercentage": models.FloatField{
			String:  "Percentage Matched",
			Compute: h.AccountMove().Methods().ComputeMatchedPercentage(),
			Depends: []string{"Lines", "Lines.Debit", "Lines.Credit", "Lines.MatchedDebits",
				"Lines.MatchedDebits.Amount", "Lines.MatchedCredits", "Lines.MatchedCredits.Amount",
				"Lines.Account", "Lines.Account.UserType", "Lines.Account.UserType.Type"},
			Stored: true,
			Help:   "Technical field used in cash basis method"},
		"StatementLine": models.Many2OneField{
			String:        "Bank statement line reconciled with this entry",
			RelationModel: h.AccountBankStatementLine(),
			Index:         true,
			NoCopy:        true,
			ReadOnly:      true},
		"DummyAccount": models.Many2OneField{
			String:        "Account",
			RelationModel: h.AccountAccount(),
			Related:       "Lines.Account"},
	})

	h.AccountMove().Methods().NameGet().Extend("",
		func(rs m.AccountMoveSet) string {
			if rs.State() == "draft" {
				return fmt.Sprintf("* %d", rs.ID())
			}
			return rs.Name()
		})

	h.AccountMove().Methods().AmountCompute().DeclareMethod(
		`AmountCompute`,
		func(rs m.AccountMoveSet) m.AccountMoveData {
			data := h.AccountMove().NewData()
			total := 0.0
			for _, line := range rs.Lines().Records() {
				total += line.Debit()
			}
			data.SetAmount(total)
			return data
		})

	h.AccountMove().Methods().ComputeMatchedPercentage().DeclareMethod(
		`Compute the percentage to apply for cash basis method. This value is relevant only for moves that
			  involve journal items on receivable or payable accounts.`,
		func(rs m.AccountMoveSet) m.AccountMoveData {
			data := h.AccountMove().NewData()
			var totalAmount float64
			var totalReconciled float64
			for _, line := range rs.Lines().Records() {
				if !strutils.IsIn(line.Account().UserType().Type(), "receivable", "payable") {
					continue
				}
				totalAmount += math.Abs(line.Debit() - line.Credit())
				for _, partialLine := range line.MatchedDebits().Union(line.MatchedCredits()).Records() {
					totalReconciled += partialLine.Amount()
				}
			}
			if nbutils.IsZero(totalAmount, rs.Currency().Rounding()) {
				data.SetMatchedPercentage(1.0)
			} else {
				data.SetMatchedPercentage(totalReconciled / totalAmount)
			}
			return data
		})

	h.AccountMove().Methods().ComputeCurrency().DeclareMethod(
		`ComputeCurrency`,
		func(rs m.AccountMoveSet) m.AccountMoveData {
			return h.AccountMove().NewData().SetCurrency(
				h.Currency().Coalesce(
					rs.Company().Currency(),
					h.User().NewSet(rs.Env()).CurrentUser().Company().Currency()))
		})

	h.AccountMove().Methods().ComputePartner().DeclareMethod(
		`ComputePartner`,
		func(rs m.AccountMoveSet) m.AccountMoveData {
			data := h.AccountMove().NewData()
			partner := h.Partner().NewSet(rs.Env())
			for _, line := range rs.Lines().Records() {
				partner = partner.Union(line.Partner())
			}
			if partner.Len() == 1 {
				data.SetPartner(partner)
			}
			return data
		})

	h.AccountMove().Methods().FieldsViewGet().Extend("",
		func(rs m.AccountMoveSet, args webdata.FieldsViewGetParams) *webdata.FieldsViewData {
			res := rs.Super().FieldsViewGet(args)
			if rs.Env().Context().GetBool("vat_domain") {
				res.Fields["line_ids"].Views["tree"].(*webdata.FieldsViewData).Fields["tax_line_id"].Domain = "[('tag_ids', 'in', [self.env.ref(self._context.get('vat_domain')).id])]"
				//tovalid is this correct? from res['fields']['line_ids']['views']['tree']['fields']['tax_line_id']['domain'] = [('tag_ids', 'in', [self.env.ref(self._context.get('vat_domain')).id])]
			}
			return res
		})

	h.AccountMove().Methods().Create().Extend("",
		func(rs m.AccountMoveSet, data m.AccountMoveData) m.AccountMoveSet {
			move := rs.Super().
				WithContext("check_move_validity", false).
				WithContext("partner_id", data.Partner().ID()).
				Create(data)
			move.AssertBalanced()
			return move
		})

	h.AccountMove().Methods().Write().Extend("",
		func(rs m.AccountMoveSet, data m.AccountMoveData) bool {
			if data.Lines().IsEmpty() {
				return rs.Super().Write(data)
			}
			res := rs.Super().WithContext("check_move_validity", false).Write(data)
			rs.AssertBalanced()
			return res
		})

	h.AccountMove().Methods().Post().DeclareMethod(
		`Post`,
		func(rs m.AccountMoveSet) bool {
			invoice := rs.Env().Context().Get("invoice").(m.AccountInvoiceSet)
			rs.PostValidate()
			for _, move := range rs.Records() {
				move.Lines().CreateAnalyticLines()
				if move.Name() != "/" {
					continue
				}
				newName := ""
				journal := move.Journal()
				if val := invoice.MoveName(); val != "" && val != "/" {
					newName = invoice.MoveName()
				} else {
					if sequence := journal.EntrySequence(); sequence.IsNotEmpty() {
						// If invoice is actually refund and journal has a refund_sequence then use that one or use the regular one
						if strutils.IsIn(invoice.Type(), "out_refund", "in_refund") && journal.RefundSequence() {
							sequence = journal.RefundEntrySequence()
							if sequence.IsEmpty() {
								panic(rs.T(`Please define a sequence for the refunds`))
							}
						}
						newName = sequence.WithContext("ir_sequence_date", move.Date()).NextByID()

					} else {
						panic(rs.T(`Please define a sequence on the journal.`))
					}
				}
				if newName != "" {
					move.SetName(newName)
				}
			}
			return rs.Write(h.AccountMove().NewData().SetState("posted"))
		})

	h.AccountMove().Methods().ButtonCancel().DeclareMethod(
		`ButtonCancel`,
		func(rs m.AccountMoveSet) bool {
			for _, move := range rs.Records() {
				if !move.Journal().UpdatePosted() {
					panic(rs.T(`You cannot modify a posted entry of this journal.\nFirst you should set the journal to allow cancelling entries.`))
				}
			}
			if len(rs.Ids()) > 0 {
				rs.CheckLockDate()
				h.AccountMove().Search(rs.Env(), q.AccountMove().ID().In(rs.Ids())).Write(h.AccountMove().NewData().SetState("draft"))
				rs.Collection().InvalidateCache()
			}
			rs.CheckLockDate()
			return true
		})

	h.AccountMove().Methods().Unlink().Extend("",
		func(rs m.AccountMoveSet) int64 {
			for _, move := range rs.Records() {
				// check the lock date + check if some entries are reconciled
				move.Lines().UpdateCheck()
				move.Lines().Unlink()
			}
			return rs.Super().Unlink()
		})

	h.AccountMove().Methods().PostValidate().DeclareMethod(
		`PostValidate`,
		func(rs m.AccountMoveSet) bool {
			for _, move := range rs.Records() {
				for _, x := range move.Lines().Records() {
					if !x.Company().Equals(move.Company()) {
						panic(rs.T(`Cannot create moves for different companies.`))
					}
				}
			}
			rs.AssertBalanced()
			return rs.CheckLockDate()
		})

	h.AccountMove().Methods().CheckLockDate().DeclareMethod(
		`CheckLockDate`,
		func(rs m.AccountMoveSet) bool {
			for _, move := range rs.Records() {
				lockDate := move.Company().FiscalyearLockDate()
				if val := move.Company().PeriodLockDate(); val.Greater(lockDate) {
					lockDate = val
				}
				hasgroup := h.User().NewSet(rs.Env()).CurrentUser().HasGroup("account.group_account_manager")
				if hasgroup {
					lockDate = move.Company().FiscalyearLockDate()
				}
				if move.Date().LowerEqual(lockDate) {
					if hasgroup {
						panic(rs.T(`You cannot add/modify entries prior to and inclusive of the lock date %s`, lockDate))
					} else {
						panic(rs.T(`You cannot add/modify entries prior to and inclusive of the lock date %s. Check the company settings or ask someone with the 'Adviser' role`, lockDate))
					}
				}
			}
			return true
		})

	h.AccountMove().Methods().AssertBalanced().DeclareMethod(
		`AssertBalanced`,
		func(rs m.AccountMoveSet) bool {
			if len(rs.Ids()) == 0 {
				return true
			}
			prec := decimalPrecision.GetPrecision("Account").Precision
			if prec < 5 {
				prec = 5
			}
			var moves []interface{}
			rs.Env().Cr().Select(&moves, `
					SELECT      move_id
			      	FROM        account_move_line
			      	WHERE       move_id in %s
			      	GROUP BY    move_id
			      	HAVING      abs(sum(debit) - sum(credit)) > %s
			      `, rs.Ids(), math.Pow10(-int(prec)))
			if len(moves) != 0 {
				panic(rs.T(`Cannot create unbalanced journal entry.`))
			}
			return true
		})

	h.AccountMove().Methods().ReverseMove().DeclareMethod(
		`ReverseMove`,
		func(rs m.AccountMoveSet, date dates.Date, journal m.AccountJournalSet) m.AccountMoveSet {
			rs.EnsureOne()
			reversedMove := rs.Copy(h.AccountMove().NewData().
				SetDate(date).
				SetJournal(h.AccountJournal().Coalesce(journal, rs.Journal())).
				SetRef(rs.T(`reversal of: %s`, rs.Name())))
			for _, acmLine := range reversedMove.Lines().WithContext("check_move_validity", false).Records() {
				acmLine.Write(h.AccountMoveLine().NewData().
					SetDebit(acmLine.Credit()).
					SetCredit(acmLine.Debit()).
					SetAmountCurrency(-acmLine.AmountCurrency()))
			}
			return reversedMove
		})

	h.AccountMove().Methods().ReverseMoves().DeclareMethod(
		`ReverseMoves`,
		func(rs m.AccountMoveSet, date dates.Date, journal m.AccountJournalSet) m.AccountMoveSet {
			if date.IsZero() {
				date = dates.Today()
			}
			reversedMoves := h.AccountMove().NewSet(rs.Env())
			for _, acMove := range rs.Records() {
				reversedMoves = reversedMoves.Union(acMove.ReverseMove(date, journal))
			}
			if reversedMoves.IsNotEmpty() {
				reversedMoves.PostValidate()
				reversedMoves.Post()
				return reversedMoves
			}
			return h.AccountMove().NewSet(rs.Env())
		})

	h.AccountMove().Methods().OpenReconcileView().DeclareMethod(
		`OpenReconcileView`,
		func(rs m.AccountMoveSet) *actions.Action {

			return rs.Lines().OpenReconcileView()
		})

	h.AccountMoveLine().DeclareModel()
	h.AccountMoveLine().SetDefaultOrder("Date DESC", "ID DESC")

	h.AccountMoveLine().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:   "Label",
			Required: true},
		"Quantity": models.FloatField{
			Digits: decimalPrecision.GetPrecision("Product Unit of Measure"),
			Help: `The optional quantity expressed by this line, eg: number of product sold.
The quantity is not a legal requirement but is very useful for some reports.`},
		"ProductUom": models.Many2OneField{
			String:        "Unit of Measure",
			RelationModel: h.ProductUom()},
		"Product": models.Many2OneField{
			String:        "Product",
			RelationModel: h.ProductProduct()},
		"Debit": models.FloatField{
			Default: models.DefaultValue(0.0)},
		"Credit": models.FloatField{
			Default: models.DefaultValue(0.0)},
		"Balance": models.FloatField{
			Compute: h.AccountMoveLine().Methods().ComputeBalance(),
			Stored:  true,
			Depends: []string{"Debit", "Credit"},
			Help:    "Technical field holding the debit - credit in order to open meaningful graph views from reports"},
		"DebitCashBasis": models.FloatField{
			Compute: h.AccountMoveLine().Methods().ComputeCashBasis(),
			Depends: []string{"Debit", "Credit", "Move.MatchedPercentage", "Move.Journal"},
			Stored:  true},
		"CreditCashBasis": models.FloatField{
			Compute: h.AccountMoveLine().Methods().ComputeCashBasis(),
			Depends: []string{"Debit", "Credit", "Move.MatchedPercentage", "Move.Journal"},
			Stored:  true},
		"BalanceCashBasis": models.FloatField{
			Compute: h.AccountMoveLine().Methods().ComputeCashBasis(),
			Depends: []string{"Debit", "Credit", "Move.MatchedPercentage", "Move.Journal"},
			Stored:  true,
			Help: `Technical field holding the debit_cash_basis - credit_cash_basis in order to open meaningful graph
views from reports`},
		"AmountCurrency": models.FloatField{
			Default:    models.DefaultValue(0.0),
			Constraint: h.AccountMoveLine().Methods().CheckCurrencyAccountAmount(),
			Help:       "The amount expressed in an optional other currency if it is a multi-currency entry."},
		"CompanyCurrency": models.Many2OneField{
			RelationModel: h.Currency(),
			Related:       "Company.Currency",
			ReadOnly:      true,
			Help:          "Utility field to express amount currency"},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency(),
			Default: func(env models.Environment) interface{} {
				if env.Context().HasKey("default_journal_id") {
					return h.AccountJournal().Browse(env, []int64{env.Context().GetInteger("default_journal_id")}).Currency()
				}
				return h.Currency().NewSet(env)
			},
			Constraint: h.AccountMoveLine().Methods().CheckCurrencyAccountAmount(),
			Help:       "The optional other currency if it is a multi-currency entry."},
		"AmountResidual": models.FloatField{
			String:  "Residual Amount",
			Compute: h.AccountMoveLine().Methods().ComputeAmountResidual(),
			Stored:  true,
			Depends: []string{"Debit", "Credit", "AmountCurrency", "Currency", "MatchedDebits", "MatchedCredits",
				"MatchedDebits.Amount", "MatchedCredits.Amount", "Account.Currency", "Move.State"},
			Help: "The residual amount on a journal item expressed in the company currency."},
		"AmountResidualCurrency": models.FloatField{
			String:  "Residual Amount in Currency",
			Compute: h.AccountMoveLine().Methods().ComputeAmountResidual(),
			Stored:  true,
			Depends: []string{"Debit", "Credit", "AmountCurrency", "Currency", "MatchedDebits", "MatchedCredits",
				"MatchedDebits.Amount", "MatchedCredits.Amount", "Account.Currency", "Move.State"},
			Help: "The residual amount on a journal item expressed in its currency (possibly not the company currency)."},
		"Account": models.Many2OneField{
			RelationModel: h.AccountAccount(),
			Required:      true,
			Index:         true,
			OnDelete:      models.Cascade,
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Default: func(env models.Environment) interface{} {
				if env.Context().HasKey("account_id") {
					return h.AccountAccount().Browse(env, []int64{env.Context().GetInteger("account_id")})
				}
				return h.AccountAccount().NewSet(env)
			},
			Constraint: h.AccountMoveLine().Methods().CheckCurrencyAccountAmount()},
		"Move": models.Many2OneField{
			String:        "Journal Entry",
			RelationModel: h.AccountMove(),
			OnDelete:      models.Cascade,
			Help:          "The move of this entry line.",
			Index:         true,
			Required:      true},
		"Narration": models.TextField{
			Related: "Move.Narration"},
		"Ref": models.CharField{
			String: "Reference",
			NoCopy: true,
			Index:  true},
		"Payment": models.Many2OneField{
			String:        "Originator Payment",
			RelationModel: h.AccountPayment(),
			Help:          "Payment that created this entry"},
		"Statement": models.Many2OneField{
			RelationModel: h.AccountBankStatement(),
			Help:          "The bank statement used for bank reconciliation",
			Index:         true,
			NoCopy:        true},
		"Reconciled": models.BooleanField{
			Compute: h.AccountMoveLine().Methods().ComputeAmountResidual(),
			Depends: []string{"Debit", "Credit", "AmountCurrency", "Currency", "MatchedDebits", "MatchedCredits",
				"MatchedDebits.Amount", "MatchedCredits.Amount", "Account.Currency", "Move.State"},
			Stored: true},
		"FullReconcile": models.Many2OneField{
			String:        "Matching Number",
			RelationModel: h.AccountFullReconcile(),
			NoCopy:        true},
		"MatchedDebits": models.One2ManyField{
			RelationModel: h.AccountPartialReconcile(),
			ReverseFK:     "CreditMove",
			JSON:          "matched_debit_ids",
			Help:          "Debit journal items that are matched with this journal item."},
		"MatchedCredits": models.One2ManyField{
			RelationModel: h.AccountPartialReconcile(),
			ReverseFK:     "DebitMove",
			JSON:          "matched_credit_ids",
			Help:          "Credit journal items that are matched with this journal item."},
		"Journal": models.Many2OneField{
			RelationModel: h.AccountJournal(),
			Related:       "Move.Journal",
			Index:         true,
			NoCopy:        true},
		"Blocked": models.BooleanField{
			String:  "No Follow-up",
			Default: models.DefaultValue(false),
			Help:    "You can check this box to mark this journal item as a litigation with the associated partner"},
		"DateMaturity": models.DateField{
			String:   "Due date",
			Index:    true,
			Required: true,
			Help: `This field is used for payable and receivable journal entries.
You can put the limit date for the payment of this line.`},
		"Date": models.DateField{
			Related: "Move.Date",
			Index:   true,
			NoCopy:  true},
		"AnalyticLines": models.One2ManyField{
			RelationModel: h.AccountAnalyticLine(),
			ReverseFK:     "Move",
			JSON:          "analytic_line_ids"},
		"Taxes": models.Many2ManyField{
			RelationModel: h.AccountTax(),
			JSON:          "tax_ids"},
		"TaxLine": models.Many2OneField{
			String:        "Originator tax",
			RelationModel: h.AccountTax(),
			OnDelete:      models.Restrict},
		"AnalyticAccount": models.Many2OneField{
			String:        "Analytic Account",
			RelationModel: h.AccountAnalyticAccount()},
		"AnalyticTags": models.Many2ManyField{
			String:        "Analytic tags",
			RelationModel: h.AccountAnalyticTag(),
			JSON:          "analytic_tag_ids"},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Related:       "Account.Company"},
		"Counterpart": models.CharField{
			Compute: h.AccountMoveLine().Methods().ComputeCounterpart(),
			Help: `Compute the counter part accounts of this journal item for this journal entry.
This can be needed in reports.`},
		"Invoice": models.Many2OneField{
			RelationModel: h.AccountInvoice()},
		"Partner": models.Many2OneField{
			RelationModel: h.Partner(),
			OnDelete:      models.Restrict},
		"UserType": models.Many2OneField{
			RelationModel: h.AccountAccountType(),
			Related:       "Account.UserType",
			Index:         true},
		"TaxExigible": models.BooleanField{
			String:  "Appears in VAT report",
			Default: models.DefaultValue(true),
			Help: `Technical field used to mark a tax line as exigible in the vat report or not
(only exigible journal items are displayed). By default all new journal items are directly exigible,
but with the module account_tax_cash_basis, some will become exigible only when the payment is recorded`},
	})

	h.AccountMoveLine().AddSQLConstraint("credit_debit1", "CHECK (credit*debit=0)",
		"Wrong credit or debit value in accounting entry !")
	h.AccountMoveLine().AddSQLConstraint("credit_debit2", "CHECK (credit+debit>=0)",
		"Wrong credit or debit value in accounting entry !")

	h.AccountMoveLine().Methods().Init().DeclareMethod(
		`Init change index on partner_id to a multi-column index on (partner_id, ref), the new index will behave in the
			      same way when we search on partner_id, with the addition of being optimal when having a query that will
			      search on partner_id and ref at the same time (which is the case when we open the bank reconciliation widget)`,
		func(rs m.AccountMoveLineSet) {
			cr := rs.Env().Cr()
			cr.Execute(`DROP INDEX IF EXISTS account_move_line_partner_id_index`)
			var out []interface{}
			cr.Select(&out, `SELECT indexname FROM pg_indexes WHERE indexname = account_move_line_partner_id_ref_idx`)
			if len(out) == 0 {
				cr.Execute(``)
			}
		})

	h.AccountMoveLine().Methods().ComputeAmountResidual().DeclareMethod(
		`ComputeAmountResidual Computes the residual amount of a move line from a reconciliable account in the company currency and the line's currency.
			      This amount will be 0 for fully reconciled lines or lines from a non-reconciliable account, the original line amount
			      for unreconciled lines, and something in-between for partially reconciled lines.`,
		func(rs m.AccountMoveLineSet) m.AccountMoveLineData {
			data := h.AccountMoveLine().NewData()
			if !rs.Account().Reconcile() {
				data.SetReconciled(false).
					SetAmountResidual(0).
					SetAmountResidualCurrency(0)
				return data
			}
			// amounts in the partial reconcile table aren't signed, so we need to use abs()
			amount := math.Abs(rs.Debit() - rs.Credit())
			amountResidualCurrency := math.Abs(rs.AmountCurrency())
			sign := -1.0
			if rs.Debit()-rs.Credit() > 0 {
				sign = 1
			}
			if rs.Debit() == 0.0 && rs.Credit() == 0.0 && rs.AmountCurrency() != 0.0 && rs.Currency().IsNotEmpty() {
				// residual for exchange rate entries
				sign = -1
				if nbutils.Compare(rs.AmountCurrency(), 0, rs.Currency().Rounding()) == 1 {
					sign = 1
				}
			}
			for _, partialLine := range rs.MatchedCredits().Union(rs.MatchedDebits()).Records() {
				// If line is a credit (sign = -1) we:
				//  - subtract matched_debit_ids (partial_line.credit_move_id == line)
				//  - add matched_credit_ids (partial_line.credit_move_id != line)
				// If line is a debit (sign = 1), do the opposite.
				signPartialLine := sign
				if partialLine.CreditMove().Equals(rs) {
					signPartialLine = sign * -1
				}
				amount += signPartialLine * partialLine.Amount()
				// getting the date of the matched item to compute the amount_residual in currency
				if rs.Currency().IsNotEmpty() {
					if partialLine.Currency().Equals(rs.Currency()) {
						amount += signPartialLine * partialLine.AmountCurrency()
					} else {
						rate := 0.0
						if rs.Balance() != 0.0 && rs.AmountCurrency() != 0.0 {
							rate = rs.AmountCurrency() / rs.Balance()
						} else {
							date := partialLine.DebitMove().Date()
							if partialLine.DebitMove().Equals(rs) {
								date = partialLine.CreditMove().Date()
							}
							rs.Currency().WithContext("date", date).Rate()
						}
						amountResidualCurrency += signPartialLine * rs.Currency().Round(partialLine.Amount()*rate)
					}
				}
			}
			// computing the `reconciled` field. As we book exchange rate difference on each partial matching,
			// we can only check the amount in company currency
			reconciled := false
			digitsRoundingPrecision := rs.Company().Currency().Rounding()
			if nbutils.IsZero(amount, digitsRoundingPrecision) {
				if rs.Currency().IsNotEmpty() && rs.AmountCurrency() != 0.0 {
					if nbutils.IsZero(amountResidualCurrency, rs.Currency().Rounding()) {
						reconciled = true
					}
				} else {
					reconciled = true
				}
			}

			data.SetReconciled(reconciled).
				SetAmountResidual(rs.Company().Currency().Round(amount * sign)).
				SetAmountResidualCurrency(rs.Currency().Round(amountResidualCurrency * sign))
			return data
		})

	h.AccountMoveLine().Methods().ComputeBalance().DeclareMethod(
		`ComputeBalance`,
		func(rs m.AccountMoveLineSet) m.AccountMoveLineData {
			return h.AccountMoveLine().NewData().SetBalance(rs.Debit() - rs.Credit())
		})

	h.AccountMoveLine().Methods().ComputeCashBasis().DeclareMethod(
		`ComputeCashBasis`,
		func(rs m.AccountMoveLineSet) m.AccountMoveLineData {
			data := h.AccountMoveLine().NewData()
			if strutils.IsIn(rs.Journal().Type(), "sale", "purchase") {
				data.SetDebitCashBasis(rs.Debit() * rs.Move().MatchedPercentage())
				data.SetCreditCashBasis(rs.Credit() * rs.Move().MatchedPercentage())
			} else {
				data.SetDebitCashBasis(rs.Debit())
				data.SetCreditCashBasis(rs.Credit())
			}
			data.SetBalanceCashBasis(data.DebitCashBasis() - data.CreditCashBasis())
			return data
		})

	h.AccountMoveLine().Methods().ComputeCounterpart().DeclareMethod(
		`ComputeCounterpart`,
		func(rs m.AccountMoveLineSet) m.AccountMoveLineData {
			var counterpart []string
			for _, line := range rs.Move().Lines().Records() {
				if line.Account().Code() != rs.Account().Code() {
					counterpart = append(counterpart, line.Account().Code())
				}
			}
			if len(counterpart) > 2 {
				counterpart = append(counterpart[:2], "...")
			}
			return h.AccountMoveLine().NewData().SetCounterpart(strings.Join(counterpart, ","))
		})

	h.AccountMoveLine().Methods().CheckCurrencyAccountAmount().DeclareMethod(
		`CheckCurrency`,
		func(rs m.AccountMoveLineSet) {
			if rs.Account().Currency().IsNotEmpty() && (rs.Currency().IsEmpty() || !rs.Currency().Equals(rs.Account().Currency())) {
				panic(rs.T(`The selected account of your Journal Entry forces to provide a secondary currency. You should remove the secondary currency on the account.`))
			}
			if rs.AmountCurrency() != 0.0 && rs.Currency().IsEmpty() {
				panic(rs.T(`You cannot create journal items with a secondary currency without filling both 'currency' and 'amount currency' field.`))
			}
			if (rs.AmountCurrency() > 0.0 && rs.Credit() > 0.0) || (rs.AmountCurrency() < 0.0 && rs.Debit() > 0.0) {
				panic(rs.T(`The amount expressed in the secondary currency must be positive when account is debited and negative when account is credited.`))
			}
		})

	h.AccountMoveLine().Methods().GetDataForManualReconciliationWidget().DeclareMethod(
		`GetDataForManualReconciliationWidget Returns the data required for the invoices & payments matching of partners/accounts.
			      If an argument is None, fetch all related reconciliations. Use [] to fetch nothing.`,
		func(rs m.AccountMoveLineSet, partners m.PartnerSet, accounts m.AccountAccountSet) *accounttypes.DataForReconciliationWidget {
			var out accounttypes.DataForReconciliationWidget
			out.Customers = rs.GetDataForManualReconciliation("partner", partners.Ids(), "receivable")
			out.Suppliers = rs.GetDataForManualReconciliation("partner", partners.Ids(), "payable")
			out.Accounts = rs.GetDataForManualReconciliation("account", accounts.Ids(), "")
			return &out
		})

	h.AccountMoveLine().Methods().GetDataForManualReconciliation().DeclareMethod(
		`GetDataForManualReconciliation Returns the data required for the invoices & payments matching of partners/accounts (list of dicts).
			      If no res_ids is passed, returns data for all partners/accounts that can be reconciled.

			      :param res_type: either 'partner' or 'account'
			      :param res_ids: ids of the partners/accounts to reconcile, use None to fetch data indiscriminately
			          of the id, use [] to prevent from fetching any data at all.
			      :param account_type: if a partner is both customer and vendor, you can use 'payable' to reconcile
			          the vendor-related journal entries and 'receivable' for the customer-related entries.`,
		func(rs m.AccountMoveLineSet, resType string, resIds []int64, accountType string) []map[string]interface{} {
			// error handling
			if resIds != nil && len(resIds) == 0 {
				// Note : this short-circuiting is better for performances, but also required
				// since postgresql doesn't implement empty list (so 'AND id in ()' is useless)
				return []map[string]interface{}{}
			}
			if !strutils.IsIn(resType, "partner", "account") {
				panic(rs.T(`GetDataForManualReconciliation: Parameter invalid: resType should be "%s" or "%s". (current: "%s")`, "partner", "account", resType))
			}
			if !strutils.IsIn(accountType, "payable", "receivable") {
				panic(rs.T(`GetDataForManualReconciliation: Parameter invalid: accountType should be "%s", "%s" or empty. (current: "%s")`, "payable", "receivable", resType))
			}

			//inits
			isPartner := resType == "partner"
			resAlias := "a"
			if isPartner {
				resAlias = "p"
			}

			// big query of doom - parameters
			BQODParams := []interface{}{ //set defaults
				" ",      //0
				" ",      //1
				resAlias, //2
				" ",      //3
				`AND at.type <> 'payable' AND at.type <> 'receivable'`, //4
				" ", //5
				" ", //6
				strconv.Itoa(int(h.User().NewSet(rs.Env()).CurrentUser().Currency().ID())), //7
				" ",      //8
				" ",      //9
				" ",      //10
				resAlias, //11
				resAlias, //12
			}
			if isPartner {
				BQODParams[0] = `partner_id, partner_name,`
				BQODParams[1] = `p.id AS partner_id, p.name AS partner_name,`
				BQODParams[3] = `RIGHT JOIN res_partner p ON (l.partner_id = p.id)`
				BQODParams[4] = ` `
				BQODParams[8] = `AND l.partner_id = p.id`
				BQODParams[9] = `AND l.partner_id = p.id`
				BQODParams[10] = `l.partner_id, p.id,`
			}
			if accountType != "" {
				BQODParams[5] = `AND at.type = :accountType `
			}
			if len(resIds) > 0 {
				BQODParams[6] = `AND ` + resAlias + `.id in :resIds "`
			}
			query := fmt.Sprintf(`
			      SELECT account_id, account_name, account_code, max_date, %s 
			             to_char(last_time_entries_checked, 'YYYY-MM-DD') AS last_time_entries_checked
			      FROM (
			              SELECT %s
			                  %s.last_time_entries_checked AS last_time_entries_checked,
			                  a.id AS account_id,
			                  a.name AS account_name,
			                  a.code AS account_code,
			                  MAX(l.write_date) AS max_date
			              FROM
			                  account_move_line l
			                  RIGHT JOIN account_account a ON (a.id = l.account_id)
			                  RIGHT JOIN account_account_type at ON (at.id = a.user_type_id)
			                  %s
			              WHERE
			                  a.reconcile IS TRUE
			                  AND l.full_reconcile_id is NULL
			                  %s
							  %s
							  %s
			                  AND l.company_id =%s
			                  AND EXISTS (
			                      SELECT NULL
			                      FROM account_move_line l
			                      WHERE l.account_id = a.id
								  %s
			                      AND l.amount_residual > 0
			                  )
			                  AND EXISTS (
			                      SELECT NULL
			                      FROM account_move_line l
			                      WHERE l.account_id = a.id
								  %s
			                      AND l.amount_residual < 0
			                  )
			              GROUP BY %s a.id, a.name, a.code, %s.last_time_entries_checked
			              ORDER BY %s.last_time_entries_checked
			          ) as s
			      WHERE (last_time_entries_checked IS NULL OR max_date > last_time_entries_checked)
			  `, BQODParams...)
			paramMap := map[string]interface{}{
				"accountType": accountType,
				"resIds":      resIds,
			}
			type RowType struct {
				accountId   int64
				accountName string
				accountCode string
				maxDate     dates.Date
				partnerId   int64
				partnerName string
			}
			var outputRows []RowType

			// Compile and exec query
			query, args, err := sqlx.Named(query, paramMap)
			if err != nil {
				panic(rs.T(`%s: Error, could not compile query, please refer to an administrator`, "GetDataForManualReconciliation"))
			}
			rs.Env().Cr().Get(&outputRows, query, args)

			// Apply ir_rules by filtering out
			var ids []int64
			for _, outputRows := range outputRows {
				ids = append(ids, outputRows.accountId)
			}
			allowedIds := h.AccountAccount().Browse(rs.Env(), ids).Ids() //tovalid is this needed that way? getting all ids after a browse of those ids?
			var rows []RowType
			for _, row := range outputRows {
				for _, id := range allowedIds {
					if row.accountId == id {
						rows = append(rows, row)
						break
					}
				}
			}
			if isPartner {
				ids = []int64{}
				for _, outputRows := range outputRows {
					ids = append(ids, outputRows.partnerId)
				}
				allowedIds = h.Partner().Browse(rs.Env(), ids).Ids()
				rows = []RowType{}
				for _, row := range outputRows {
					for _, id := range allowedIds {
						if row.partnerId == id {
							rows = append(rows, row)
							break
						}
					}
				}
			}

			// Fetch other data and store to map
			var out []map[string]interface{}
			for _, row := range rows {
				o := make(map[string]interface{})
				account := h.AccountAccount().BrowseOne(rs.Env(), row.accountId)
				o["currency_id"] = h.Currency().Coalesce(account.Currency(), account.Company().Currency()).ID()
				o["reconciliation_proposition"] = rs.GetReconciliationProposition(account, h.Partner().BrowseOne(rs.Env(), row.partnerId))
				o["account_id"] = row.accountId
				o["account_name"] = row.accountName
				o["account_code"] = row.accountCode
				o["max_date"] = row.maxDate
				o["partner_id"] = row.partnerId
				o["partner_name"] = row.partnerName
				out = append(out, o)
			}

			return out
		})

	h.AccountMoveLine().Methods().GetReconciliationProposition().DeclareMethod(
		`Returns two lines whose amount are opposite`,
		func(rs m.AccountMoveLineSet, account m.AccountAccountSet, partner m.PartnerSet) []map[string]interface{} {
			// Get pairs
			params := map[string]interface{}{
				"accountId": account.ID(),
			}
			partnerIDCondition := ""
			if partner.IsNotEmpty() {
				partnerIDCondition = `AND a.partner_id = :partnerID AND b.partner_id = :partnerID`
				params["partnerID"] = partner.ID()
			}
			query, args, err := sqlx.Named(`
			          SELECT a.id, b.id
			          FROM account_move_line a, account_move_line b
			          WHERE a.amount_residual = -b.amount_residual
			          AND NOT a.reconciled AND NOT b.reconciled
			          AND a.account_id = :accountID AND b.account_id = :accountID
			          `+partnerIDCondition+`
			          ORDER BY a.date asc
			          LIMIT 10
			      `, params)
			if err != nil {
				panic(rs.T(`%s: Error, could not compile query, please refer to an administrator`, "GetReconciliationProposition"))
			}
			type pairType struct {
				a int64
				b int64
			}
			var pairsOut []pairType
			rs.Env().Cr().Select(&pairsOut, query, args...)

			// Apply ir_rules by filtering out
			var allIds []int64
			for _, pair := range pairsOut {
				allIds = append(allIds, pair.a, pair.b)
			}
			allowed := h.AccountMoveLine().Browse(rs.Env(), allIds).Ids() //tovalid is this needed that way? getting all ids after a browse of those ids?
			var pairs []pairType
			for _, pair := range pairsOut {
				aIsIn := false
				bIsIn := false
				for _, id := range allowed {
					if pair.a == id {
						aIsIn = true
					}
					if pair.b == id {
						bIsIn = true
					}
					if aIsIn && bIsIn {
						pairs = append(pairs, pair)
						break
					}
				}
			}

			// Return lines formatted
			if len(pairs) > 0 {
				targetCurrency := rs.Company().Currency()
				if rs.Currency().IsNotEmpty() && rs.AmountCurrency() != 0.0 {
					targetCurrency = rs.Currency()
				}
				lines := rs.Browse([]int64{pairs[0].a, pairs[0].b})
				return lines.PrepareMoveLinesForReconciliationWidget(targetCurrency, dates.Date{})
			}
			return []map[string]interface{}{}
		})

	h.AccountMoveLine().Methods().DomainMoveLinesForReconciliation().DeclareMethod(
		`DomainMoveLinesForReconciliation Returns the domain which is common to both manual and bank statement reconciliation.

			      :param excluded_ids: list of ids of move lines that should not be fetched
			      :param str: search string`,
		func(rs m.AccountMoveLineSet, excludedIds []int64, str string) q.AccountMoveLineCondition {
			epsilon := 0.0001
			var domain q.AccountMoveLineCondition

			if len(excludedIds) > 1 {
				domain = q.AccountMoveLine().ID().NotIn(excludedIds)
			}
			if str == "" {
				return domain
			}

			strDomain := q.AccountMoveLine().
				MoveFilteredOn(q.AccountMove().Name().IContains(str).
					Or().Ref().IContains(str)).
				Or().DateMaturity().Equals(dates.ParseDate(str)).
				OrCond(q.AccountMoveLine().Name().NotEquals("/").
					And().Name().IContains(str))

			amount, _ := strconv.ParseFloat(str, 64)

			amountResidualDomain := q.AccountMoveLine().AmountResidual().LowerOrEqual(amount + epsilon).
				And().AmountResidual().GreaterOrEqual(amount - epsilon).
				OrCond(q.AccountMoveLine().AmountResidual().LowerOrEqual(-amount + epsilon).
					And().AmountResidual().GreaterOrEqual(-amount - epsilon))

			amountResidualCurrencyDomain := q.AccountMoveLine().AmountResidualCurrency().LowerOrEqual(amount + epsilon).
				And().AmountResidualCurrency().GreaterOrEqual(amount - epsilon).
				OrCond(q.AccountMoveLine().AmountResidualCurrency().LowerOrEqual(-amount + epsilon).
					And().AmountResidualCurrency().GreaterOrEqual(-amount - epsilon))

			debitDomain := q.AccountMoveLine().Debit().LowerOrEqual(amount + epsilon).
				And().Debit().GreaterOrEqual(amount - epsilon).
				OrCond(q.AccountMoveLine().Debit().LowerOrEqual(-amount + epsilon).
					And().Debit().GreaterOrEqual(-amount - epsilon))

			creditDomain := q.AccountMoveLine().Credit().LowerOrEqual(amount + epsilon).
				And().Credit().GreaterOrEqual(amount - epsilon).
				OrCond(q.AccountMoveLine().Credit().LowerOrEqual(-amount + epsilon).
					And().Credit().GreaterOrEqual(-amount - epsilon))

			amountCurrencyDomain := q.AccountMoveLine().AmountCurrency().LowerOrEqual(amount + epsilon).
				And().AmountCurrency().GreaterOrEqual(amount - epsilon).
				OrCond(q.AccountMoveLine().AmountCurrency().LowerOrEqual(-amount + epsilon).
					And().AmountCurrency().GreaterOrEqual(-amount - epsilon))

			residualDomain := amountResidualDomain.OrCond(amountResidualCurrencyDomain)
			liquidityDomain := q.AccountMoveLine().AccountFilteredOn(q.AccountAccount().InternalType().Equals("liquidity")).
				AndCond(debitDomain.OrCond(creditDomain).OrCond(amountCurrencyDomain))
			strDomain = strDomain.OrCond(residualDomain.OrCond(liquidityDomain))

			// When building a domain for the bank statement reconciliation, if there's no partner
			// and a search string, search also a match in the partner names
			if id := rs.Env().Context().GetInteger("bank_statement_line_id"); h.AccountBankStatementLine().BrowseOne(rs.Env(), id).IsNotEmpty() {
				strDomain = strDomain.OrCond(q.AccountMoveLine().PartnerFilteredOn(q.Partner().Name().IContains(str)))
			}
			domain = domain.AndCond(strDomain)

			return domain
		})

	h.AccountMoveLine().Methods().DomainMoveLinesForManualReconciliation().DeclareMethod(
		`DomainMoveLinesForManualReconciliation Create domain criteria that are relevant to manual reconciliation.`,
		func(rs m.AccountMoveLineSet, account m.AccountAccountSet, partner m.PartnerSet, excludedIds []int64, str string) q.AccountMoveLineCondition {
			domain := q.AccountMoveLine().Reconciled().Equals(false).
				And().Account().Equals(account)
			if partner.IsNotEmpty() {
				domain = domain.And().Partner().Equals(partner)
			}
			genericDomain := rs.DomainMoveLinesForReconciliation(excludedIds, str)
			return genericDomain.AndCond(domain)
		})

	h.AccountMoveLine().Methods().GetMoveLinesForManualReconciliation().DeclareMethod(
		`GetMoveLinesForManualReconciliation Returns unreconciled move lines for an account or a partner+account,
				formatted for the manual reconciliation widget`,
		func(rs m.AccountMoveLineSet, account m.AccountAccountSet, partner m.PartnerSet, excludedIds []int64,
			str string, offset, limit int, targetCurrency m.CurrencySet) []map[string]interface{} {
			domain := rs.DomainMoveLinesForManualReconciliation(account, partner, excludedIds, str)
			lines := rs.Search(domain).Offset(offset).Limit(limit).OrderBy("date_maturity asc", "id asc")
			if targetCurrency.IsEmpty() {
				targetCurrency = h.Currency().Coalesce(account.Currency(), account.Company().Currency())
			}
			return lines.PrepareMoveLinesForReconciliationWidget(targetCurrency, dates.Date{})
		})

	h.AccountMoveLine().Methods().PrepareMoveLinesForReconciliationWidget().DeclareMethod(
		`PrepareMoveLinesForReconciliationWidget
					Returns move lines formatted for the manual/bank reconciliation widget
			      :param target_currency: currency (browse_record or ID) you want the move line debit/credit converted into
			      :param target_date: date to use for the monetary conversion`,
		func(rs m.AccountMoveLineSet, targetCurrency m.CurrencySet, targetDate dates.Date) []map[string]interface{} {
			var out []map[string]interface{}

			for _, line := range rs.Records() {
				account := line.Account()
				companyCurrency := account.Company().Currency()
				retLine := map[string]interface{}{
					"id":  line.ID(),
					"ref": line.Move().Ref(),
					//For reconciliation between statement transactions and already registered payments (eg. checks)
					//NB : we don"t use the "reconciled" field because the line we"re selecting is not the one that gets reconciled
					"already_paid":  account.InternalType() == "liquidity",
					"account_code":  account.Code(),
					"account_name":  account.Name(),
					"account_type":  account.InternalType(),
					"date_maturity": line.DateMaturity(),
					"date":          line.Date(),
					"journal_name":  line.Journal().Name(),
					"partner_id":    line.Partner().ID(),
					"partner_name":  line.Partner().Name(),
					"currency_id":   h.Currency().NewSet(rs.Env()).ID(),
				}
				if curr := line.Currency(); curr.IsNotEmpty() && line.AmountCurrency() != 0.0 {
					retLine["currency_id"] = curr
				}
				name := line.Move().Name()
				if line.Name() != "/" {
					name = name + ": " + line.Name()
				}
				retLine["name"] = name

				debit := line.Debit()
				credit := line.Credit()
				amount := line.AmountResidual()
				amountCurrency := line.AmountResidualCurrency()

				//For already reconciled lines, don't use amount_residual(_currency)
				if account.InternalType() == "liquidity" {
					amount = math.Abs(debit - credit)
					amountCurrency = math.Abs(line.AmountCurrency())
				}

				// Get right debit / credit:
				targetCurrency = h.Currency().Coalesce(targetCurrency, companyCurrency)
				lineCurrency := companyCurrency
				if line.Currency().IsNotEmpty() && amountCurrency != 0.0 {
					lineCurrency = line.Currency()
				}
				amountCurrencyStr := ""
				totalAmountCurrencyStr := ""

				// The payment currency is the invoice currency, but they are different than the company currency
				// We use the `amount_currency` computed during the invoice validation, at the invoice date
				// to avoid exchange gain/loss
				// e.g. an invoice of 100€ must be paid with 100€, whatever the company currency and the exchange rates
				totalAmount := line.AmountCurrency()
				currency := lineCurrency
				actualDebit := 0.0
				actualCredit := 0.0
				if debit > 0 {
					actualDebit = amountCurrency
				}
				if credit > 0 {
					actualCredit = -amountCurrency
				}

				if !(!lineCurrency.Equals(companyCurrency) && targetCurrency.Equals(lineCurrency)) {
					// Either:
					//  - the invoice, payment, company currencies are all the same,
					//  - the payment currency is the company currency, but the invoice currency is different,
					//  - the invoice currency is the company currency, but the payment currency is different,
					//  - the invoice, payment and company currencies are all different.
					// For the two first cases, we can simply use the debit/credit of the invoice move line, which are always in the company currency,
					// and this is what the target need.
					// For the two last cases, we can use the debit/credit which are in the company currency, and then change them to the target currency
					totalAmount = math.Abs(debit - credit)
					currency = companyCurrency
					actualDebit = 0.0
					actualCredit = 0.0
					if debit > 0 {
						actualDebit = amount
					}
					if credit > 0 {
						actualCredit = -amount
					}
				}

				if !lineCurrency.Equals(targetCurrency) {
					value := actualCredit
					if actualDebit != 0.0 {
						value = actualDebit
					}
					amountCurrencyStr = FormatLang(rs.Env(), math.Abs(value), lineCurrency)
					totalAmountCurrencyStr = FormatLang(rs.Env(), totalAmount, lineCurrency)
				}
				if !currency.Equals(targetCurrency) {
					value := targetDate
					if value.IsZero() {
						value = line.Date()
					}
					curr := currency.WithContext("date", value)
					totalAmount = curr.Compute(totalAmount, targetCurrency, true)
					actualDebit = curr.Compute(actualDebit, targetCurrency, true)
					actualCredit = curr.Compute(actualCredit, targetCurrency, true)
				}
				value := actualDebit
				if value == 0.0 {
					value = actualCredit
				}
				amountStr := FormatLang(rs.Env(), math.Abs(actualCredit), targetCurrency)
				totalAmountStr := FormatLang(rs.Env(), totalAmount, targetCurrency)
				retLine["debit"] = math.Abs(actualDebit)
				retLine["credit"] = math.Abs(actualCredit)
				retLine["amount_str"] = amountStr
				retLine["total_amount_str"] = totalAmountStr
				retLine["amount_currency_str"] = amountCurrencyStr
				retLine["total_amount_currency_str"] = totalAmountCurrencyStr
				out = append(out, retLine)
			}
			return out
		})

	h.AccountMoveLine().Methods().ProcessReconciliations().DeclareMethod(
		`ProcessReconciliations Used to validate a batch of reconciliations in a single call
			      :param data: list of dicts containing:
			          - 'type': either 'partner' or 'account'
			          - 'id': id of the affected res.partner or account.account
			          - 'mv_line_ids': ids of exisiting account.move.line to reconcile
			          - 'new_mv_line_dicts': list of dicts containing values suitable for account_move_line.create()`,
		func(rs m.AccountMoveLineSet, data []struct {
			Type             string                  `json:"type"`
			ID               int64                   `json:"id"`
			MoveLineIds      []int64                 `json:"mv_line_ids"`
			NewMoveLinesData []m.AccountMoveLineData `json:"new_mv_line_dicts"`
		}) {
			for _, datum := range data {
				if len(datum.MoveLineIds) > 0 || len(datum.NewMoveLinesData) > 1 {
					h.AccountMoveLine().Browse(rs.Env(), datum.MoveLineIds).ProcessReconciliation(datum.NewMoveLinesData)
				}
				switch datum.Type {
				case "partner":
					h.Partner().BrowseOne(rs.Env(), datum.ID)
				case "account":
					h.AccountAccount().BrowseOne(rs.Env(), datum.ID).MarkAsReconciled()
				}
			}
		})

	h.AccountMoveLine().Methods().ProcessReconciliation().DeclareMethod(
		`ProcessReconciliation Create new move lines from new_mv_line_dicts (if not empty) then call reconcile_partial on self and new move lines

			      :param new_mv_line_dicts: list of dicts containing values suitable fot account_move_line.create()`,
		func(rs m.AccountMoveLineSet, newMoveLinesData []m.AccountMoveLineData) {
			if rsLen := rs.Len(); rsLen < 1 || rsLen+len(newMoveLinesData) < 2 {
				panic(rs.T(`A reconciliation must involve at least 2 move lines.`))
			}

			// Create writeoff move lines
			if len(newMoveLinesData) == 0 {
				rs.Reconcile(h.AccountAccount().NewSet(rs.Env()), h.AccountJournal().NewSet(rs.Env()))
				return
			}
			writeoffLines := h.AccountMoveLine().NewSet(rs.Env())
			companyCurrency := rs.Account().Company().Currency()
			writeoffCurrency := h.Currency().Coalesce(rs.Currency(), companyCurrency)
			for _, data := range newMoveLinesData {
				if !writeoffCurrency.Equals(companyCurrency) {
					data.SetDebit(writeoffCurrency.Compute(data.Debit(), companyCurrency, true))
					data.SetCredit(writeoffCurrency.Compute(data.Credit(), companyCurrency, true))
				}
				writeoffLines.Union(rs.CreateWriteoff(data))
			}
			rs.Union(writeoffLines).Reconcile(h.AccountAccount().NewSet(rs.Env()), h.AccountJournal().NewSet(rs.Env()))
		})

	h.AccountMoveLine().Methods().GetPairToReconcile().DeclareMethod(
		`GetPairToReconcile`,
		func(rs m.AccountMoveLineSet) (m.AccountMoveLineSet, m.AccountMoveLineSet) {
			// field is either 'amount_residual' or 'amount_residual_currency' (if the reconciled account has a secondary currency set)
			field := "AmountResidual"
			if rs.Account().Currency().IsNotEmpty() {
				field = "AmountResidualCurrency"
			}
			rounding := rs.Company().Currency().Rounding()
			currency := rs.Currency()
			if currency.IsNotEmpty() {
				bool := true
				for _, x := range rs.Records() {
					if !(x.AmountCurrency() != 0.0 && x.Currency().Equals(currency)) {
						bool = false
					}
				}
				if bool {
					// or if all lines share the same currency
					field = "AmountResidualCurrency"
					rounding = rs.Currency().Rounding()
				}
			}
			switch rs.Env().Context().GetString("skip_full_reconcile_check") {
			case "amount_currency_excluded":
				field = "AmountResidual"
			case "amount_currency_only":
				field = "AmountResidualCurrency"
			}
			// target the pair of move in self that are the oldest
			sortedMoves := rs.Sorted(func(rs1, rs2 m.AccountMoveLineSet) bool {
				value1 := rs1.DateMaturity()
				if value1.IsZero() {
					value1 = rs1.Date()
				}
				value2 := rs2.DateMaturity()
				if value2.IsZero() {
					value2 = rs2.Date()
				}
				return value1.Greater(value2)
			})
			debit := h.AccountMoveLine().NewSet(rs.Env())
			credit := h.AccountMoveLine().NewSet(rs.Env())
			for _, aml := range sortedMoves.Records() {
				if credit.IsNotEmpty() && debit.IsNotEmpty() {
					break
				}
				valueCmp := nbutils.Compare(aml.Get(field).(float64), 0, rounding)
				if debit.IsEmpty() && valueCmp == 1 {
					debit = aml
				}
				if credit.IsEmpty() && valueCmp == -1 {
					credit = aml
				}
			}
			return debit, credit
		})

	h.AccountMoveLine().Methods().AutoReconcileLines().DeclareMethod(
		`AutoReconcileLines This function iterates recursively on the recordset given as parameter as long as it
			      can find a debit and a credit to reconcile together. It returns the recordset of the
			      account move lines that were not reconciled during the process.`,
		func(rs m.AccountMoveLineSet) m.AccountMoveLineSet {
			if rs.IsEmpty() {
				return rs
			}
			smDebitMove, smCreditMove := rs.GetPairToReconcile()
			// there is no more pair to reconcile so return what move_line are left
			if smDebitMove.IsEmpty() || smCreditMove.IsEmpty() {
				return rs
			}
			field := "AmountResidual"
			if rs.Account().Currency().IsNotEmpty() {
				field = "AmountResidualCurrency"
			}
			if smDebitMove.Debit() == 0.0 && smCreditMove.Credit() == 0.0 {
				// both debit and credit field are 0, consider the amount_residual_currency field because it's an exchange difference entry
				field = "AmountResidualCurrency"
			}
			currency := rs.Currency()
			if currency.IsNotEmpty() {
				bool := true
				for _, x := range rs.Records() {
					if !(x.AmountCurrency() != 0.0 && x.Currency().Equals(currency)) {
						bool = false
					}
				}
				if bool {
					// all the lines have the same currency, so we consider the amount_residual_currency field
					field = "AmountResidualCurrency"
				}
			}
			switch rs.Env().Context().GetString("skip_full_reconcile_check") {
			case "amount_currency_excluded":
				field = "AmountResidual"
			case "amount_currency_only":
				field = "AmountResidualCurrency"
			}

			// Reconcile the pair together
			DebitValue := smDebitMove.Get(field).(float64)
			CreditValue := smCreditMove.Get(field).(float64)
			amountReconcile := DebitValue
			if -CreditValue < amountReconcile {
				amountReconcile = -CreditValue
			}
			// Remove from recordset the one(s) that will be totally reconciled
			if amountReconcile == DebitValue {
				rs.Subtract(smDebitMove)
			}
			if amountReconcile == -CreditValue {
				rs.Subtract(smCreditMove)
			}
			// Check for the currency and amount_currency we can set
			currency = h.Currency().NewSet(rs.Env())
			amountReconcileCurrency := 0.0
			if smDebitMove.Currency().Equals(smCreditMove.Currency()) && smDebitMove.Currency().IsNotEmpty() {
				currency = smCreditMove.Currency()
				amountReconcileCurrency = smDebitMove.AmountResidualCurrency()
				if val := smCreditMove.AmountResidualCurrency(); -val < amountReconcileCurrency {
					amountReconcileCurrency = -val
				}
			}

			amountReconcile = smDebitMove.AmountResidual()
			if val := smCreditMove.AmountResidual(); -val < amountReconcile {
				amountReconcile = -val
			}

			switch rs.Env().Context().GetString("skip_full_reconcile_check") {
			case "amount_currency_excluded":
				amountReconcileCurrency = 0.0
				fallthrough
			case "amount_currency_only":
				currency = h.Currency().BrowseOne(rs.Env(), rs.Env().Context().GetInteger("manual_full_reconcile_currency_id"))
			}

			h.AccountPartialReconcile().Create(rs.Env(), h.AccountPartialReconcile().NewData().
				SetDebitMove(smDebitMove).
				SetCreditMove(smCreditMove).
				SetAmount(amountReconcile).
				SetAmountCurrency(amountReconcileCurrency).
				SetCurrency(currency))

			// Iterate process again on self
			return rs.AutoReconcileLines()
		})

	h.AccountMoveLine().Methods().Reconcile().DeclareMethod(
		`Reconcile`,
		func(rs m.AccountMoveLineSet, writeoffAccount m.AccountAccountSet, writeoffJournal m.AccountJournalSet) m.AccountMoveLineSet {
			// Empty self can happen if the user tries to reconcile entries which are already reconciled.
			// The calling method might have filtered out reconciled lines.
			if rs.IsEmpty() {
				return h.AccountMoveLine().NewSet(rs.Env())
			}

			// Perform all checks on lines
			companies := h.Company().NewSet(rs.Env())
			accounts := h.AccountAccount().NewSet(rs.Env())
			currencies := h.Currency().NewSet(rs.Env())
			for _, line := range rs.Records() {
				if line.Reconciled() {
					panic(rs.T(`You are trying to reconcile some entries that are already reconciled!`))
				}
				companies = companies.Union(line.Company())
				accounts = accounts.Union(line.Account())
				currencies = currencies.Union(line.Currency())
			}
			if companies.Len() > 1 {
				panic(rs.T(`To reconcile the entries company should be the same for all entries!`))
			}
			if accounts.Len() > 1 {
				panic(rs.T(`Entries are not of the same account!`))
			}
			if !(accounts.Reconcile() || accounts.InternalType() == "liquidity") {
				panic(rs.T(`The account %s (%s) is not marked as reconciliable !`, accounts.Name(), accounts.Code()))
			}

			// reconcile everything that can be
			remainingMoves := rs.AutoReconcileLines()

			// if writeoff_acc_id specified, then create write-off move with value the remaining amount from move in self
			if !(writeoffAccount.IsNotEmpty() && writeoffJournal.IsNotEmpty() && remainingMoves.IsNotEmpty()) {
				return h.AccountMoveLine().NewSet(rs.Env())
			}
			shareSameCurrency := !(currencies.Len() > 1)
			vals := h.AccountMoveLine().NewData().
				SetAccount(writeoffAccount).
				SetJournal(writeoffJournal)
			if !shareSameCurrency {
				vals.SetAmountCurrency(0.0)
			}
			writeoffToReconcile := remainingMoves.CreateWriteoff(vals)
			// add writeoff line to reconcile algo and finish the reconciliation
			remainingMoves = remainingMoves.Union(writeoffToReconcile).AutoReconcileLines()
			return writeoffToReconcile
		})

	h.AccountMoveLine().Methods().CreateWriteoff().DeclareMethod(
		`CreateWriteoff Create a writeoff move for the account.move.lines in self. If debit/credit is not specified in vals,
			      the writeoff amount will be computed as the sum of amount_residual of the given recordset.

			      :param vals: dict containing values suitable fot account_move_line.create(). The data in vals will
			          be processed to create bot writeoff acount.move.line and their enclosing account.move.`,
		func(rs m.AccountMoveLineSet, data m.AccountMoveLineData) m.AccountMoveLineSet {
			// Check and complete vals
			if !data.HasAccount() || !data.HasJournal() {
				panic(rs.T(`It is mandatory to specify an account and a journal to create a write-off.`))
			}
			if (data.HasDebit() && !data.HasCredit()) || (!data.HasDebit() && data.HasCredit()) {
				panic(rs.T(`Either pass both debit and credit or none.`))
			}
			if !data.HasDate() {
				value := rs.Env().Context().GetDate("date_p")
				if value.IsZero() {
					value = dates.Today()
				}
				data.SetDate(value)
			}
			if !data.HasName() {
				value := rs.Env().Context().GetString("comment")
				if value == "" {
					value = rs.T(`Write-Off`)
				}
				data.SetName(value)
			}
			if !data.HasAnalyticAccount() {
				data.SetAnalyticAccount(h.AccountAnalyticAccount().BrowseOne(rs.Env(), rs.Env().Context().GetInteger("analytic_id")))
			}

			// Compute the writeoff amount if not given
			if !data.HasCredit() { // -> && !data.HasDebit()
				amount := 0.0
				for _, r := range rs.Records() {
					amount += r.AmountResidual()
				}
				switch {
				case amount > 0.0:
					data.SetCredit(amount)
					data.SetDebit(0.0)
				case amount < 0.0:
					data.SetCredit(0.0)
					data.SetDebit(math.Abs(amount))
				case amount == 0.0:
					data.SetCredit(0.0)
					data.SetDebit(0.0)
				}
			}

			data.SetPartner(h.Partner().NewSet(rs.Env()).FindAccountingPartner(rs.Partner()))
			companyCurrency := rs.Account().Company().Currency()
			writeoffCurrency := h.Currency().Coalesce(rs.Currency(), companyCurrency)
			if rs.Env().Context().GetString("skip_full_reconcile_check") != "amount_currency_excluded" && !data.HasAmountCurrency() && !writeoffCurrency.Equals(companyCurrency) {
				data.SetCurrency(writeoffCurrency)
				sign := -1.0
				if data.Debit() > 0 {
					sign = 1.0
				}
				amount := 0.0
				for _, r := range rs.Records() {
					amount += r.AmountResidualCurrency()
				}
				data.SetAmountCurrency(sign * math.Abs(amount))
			}

			// Writeoff line in the account of self
			firstLine := rs.PrepareWriteoffFirstLine(data)

			// Writeoff line in specified writeoff account
			secondLine := rs.PrepareWriteoffSecondLine(data)

			// Create the move
			ctx := types.NewContext().WithKey("apply_taxes", true)
			writeoffMove := h.AccountMove().NewSet(rs.Env()).WithNewContext(ctx).Create(h.AccountMove().NewData().
				SetJournal(data.Journal()).
				SetDate(data.Date()).
				SetState("draft").
				SetLines(h.AccountMoveLine().Create(rs.Env(), firstLine).Union(h.AccountMoveLine().Create(rs.Env(), secondLine))))
			writeoffMove.Post()

			// Return the writeoff move.line which is to be reconciled
			return writeoffMove.Lines().Filtered(func(r m.AccountMoveLineSet) bool { return r.Account() == rs.Account() })
		})

	h.AccountMoveLine().Methods().PrepareWriteoffFirstLine().DeclareMethod(
		`PrepareWriteoffFirstLine`,
		func(rs m.AccountMoveLineSet, data m.AccountMoveLineData) m.AccountMoveLineData {
			line := data.Copy()
			line.SetAccount(rs.Account())
			if line.HasAnalyticAccount() {
				line.UnsetAnalyticAccount()
			}
			if !line.HasTaxes() {
				return line
			}

			amount := line.Credit() - line.Debit()
			_, _, amountTax, _ := line.Taxes().ComputeAll(amount, h.Currency().NewSet(rs.Env()), 1.0,
				h.ProductProduct().NewSet(rs.Env()), h.Partner().NewSet(rs.Env()))
			line.SetCredit(0.0)
			if amountTax > 0 {
				line.SetCredit(amountTax)
			}
			line.SetDebit(0.0)
			if amountTax < 0 {
				line.SetDebit(math.Abs(amountTax))
			}
			line.UnsetTaxes()
			return line
		})

	h.AccountMoveLine().Methods().PrepareWriteoffSecondLine().DeclareMethod(
		`PrepareWriteoffSecondLine`,
		func(rs m.AccountMoveLineSet, data m.AccountMoveLineData) m.AccountMoveLineData {
			line := data.Copy()
			credit, debit := line.Credit(), line.Debit()
			line.SetDebit(credit)
			line.SetCredit(debit)
			if line.HasAmountCurrency() {
				line.SetAmountCurrency(-line.AmountCurrency())
			}
			return line
		})

	h.AccountMoveLine().Methods().ComputeFullAfterBatchReconcile().DeclareMethod(
		`ComputeFullAfterBatchReconcile
				After running the manual reconciliation wizard and making full reconciliation, we need to run this method to create
			      potentially an exchange rate entry that will balance the remaining amount_residual_currency (possibly several aml).

			      This ensure that all aml in the full reconciliation are reconciled (amount_residual = amount_residual_currency = 0). `,
		func(rs m.AccountMoveLineSet) {
			var totalDebit float64
			var totalCredit float64
			var totalAmountCurrency float64
			var currency m.CurrencySet
			var aml m.AccountMoveLineSet
			var amlToBalanceCurrency m.AccountMoveLineSet
			var partialRec m.AccountPartialReconcileSet
			var partialRecSet m.AccountPartialReconcileSet
			var maxDate dates.Date

			amlToBalanceCurrency = h.AccountMoveLine().NewSet(rs.Env())
			partialRecSet = h.AccountPartialReconcile().NewSet(rs.Env())

			for _, aml := range rs.Records() {
				totalDebit += aml.Debit()
				totalCredit += aml.Credit()
				if aml.AmountResidualCurrency() != 0.0 {
					amlToBalanceCurrency = amlToBalanceCurrency.Union(aml)
				}
				if val := aml.Date(); val.Greater(maxDate) {
					maxDate = val
				}
				if currency.IsEmpty() && aml.Currency().IsNotEmpty() {
					currency = aml.Currency()
				}
				if aml.Currency().IsNotEmpty() && aml.Currency().Equals(currency) {
					totalAmountCurrency += aml.AmountCurrency()
				}
				partialRecSet = partialRecSet.Union(aml.MatchedDebits().Union(aml.MatchedCredits()))
			}

			if currency.IsNotEmpty() && amlToBalanceCurrency.IsNotEmpty() {
				var otherAml m.AccountMoveLineSet
				var otherPartialRec m.AccountPartialReconcileSet

				aml = amlToBalanceCurrency.Records()[0]
				// eventually create journal entries to book the difference due to foreign currency's exchange rate that fluctuates
				if aml.Credit() != 0.0 {
					partialRec = aml.MatchedDebits().Records()[0]
				} else {
					partialRec = aml.MatchedCredits().Records()[0]
				}
				otherAml, otherPartialRec = partialRec.WithContext("skip_full_reconcile_check", true).
					CreateExchangeRateEntry(amlToBalanceCurrency, 0.0, totalAmountCurrency, currency, maxDate)
				rs = rs.Union(otherAml)
				partialRecSet = partialRecSet.Union(otherPartialRec)
				totalAmountCurrency += aml.AmountCurrency()
			}

			//if the total debit and credit are equal, and the total amount in currency is 0, the reconciliation is full
			if nbutils.Compare(totalDebit, totalCredit, rs.Company().Currency().Rounding()) == 0 &&
				(currency.IsEmpty() || nbutils.IsZero(totalAmountCurrency, currency.Rounding())) {
				// in that case, mark the reference on the partial reconciliations and the entries
				data := h.AccountFullReconcile().NewData().
					SetPartialReconciles(partialRecSet).
					SetReconciledLines(rs).
					SetExchangeMove(aml.Move()).
					SetExchangePartialRec(partialRec)
				h.AccountFullReconcile().NewSet(rs.Env()).WithContext("check_move_validity", false).Create(data)
			}
		})

	h.AccountMoveLine().Methods().RemoveMoveReconcile().DeclareMethod(
		`RemoveMoveReconcile Undo a reconciliation`,
		func(rs m.AccountMoveLineSet) int64 {
			var recMoves m.AccountPartialReconcileSet

			if rs.IsEmpty() {
				return 0
			}
			recMoves = h.AccountPartialReconcile().NewSet(rs.Env())
			for _, aml := range rs.Records() {
				for _, invoice := range aml.Payment().Invoices().Records() {
					if invoice.ID() == rs.Env().Context().GetInteger("invoice_id") &&
						aml.Intersect(invoice.PaymentMoveLines()).IsNotEmpty() {
						aml.Payment().Write(h.AccountPayment().NewData().SetInvoices(invoice))
					}
				}
				recMoves = recMoves.Union(aml.MatchedDebits())
				recMoves = recMoves.Union(aml.MatchedCredits())
			}
			return recMoves.Unlink()
		})

	h.AccountMoveLine().Methods().Create().Extend(`
			:context's key apply_taxes: set to True if you want vals['tax_ids'] to result in the creation of move lines for taxes and eventual
				adjustment of the line amount (in case of a tax included in price).
			:context's key 'check_move_validity': check data consistency after move line creation. Eg. set to false to disable verification that the move
				debit-credit == 0 while creating the move lines composing the move.`,
		func(rs m.AccountMoveLineSet, data m.AccountMoveLineData) m.AccountMoveLineSet {
			var taxLinesData []m.AccountMoveLineData
			var newLine m.AccountMoveLineSet
			var account m.AccountAccountSet
			var journal m.AccountJournalSet
			var move m.AccountMoveSet
			var ctx *types.Context
			var amount float64
			var ok bool

			ctx = rs.Env().Context()
			amount = data.Debit() - data.Credit()
			if id := ctx.GetInteger("partner_id"); !data.HasPartner() && id != 0 {
				data.SetPartner(h.Partner().BrowseOne(rs.Env(), id))
			}
			move = data.Move()
			account = data.Account()
			if account.Deprecated() {
				panic(rs.T(`You cannot use deprecated account.`))
			}
			if !data.Date().IsZero() {
				ctx = ctx.WithKey("date", data.Date())
			}
			if data.Journal().IsNotEmpty() {
				ctx = ctx.WithKey("journal_id", data.Journal().ID())
			} else {
				ctx = ctx.WithKey("journal_id", move.Journal().ID()).
					WithKey("date", move.Date())
			}
			// we need to treat the case where a value is given in the context for period_id as a string
			if ctx.GetInteger("journal_id") == 0 && ctx.GetInteger("search_default_journal_id") != 0 {
				ctx = ctx.WithKey("journal_id", ctx.GetInteger("search_default_journal_id"))
			}
			if !ctx.HasKey("date") {
				ctx = ctx.WithKey("date", dates.Today())
			}
			journal = h.AccountJournal().Coalesce(data.Journal(), move.Journal())
			switch {
			case !data.DateMaturity().IsZero():
				break
			case !data.Date().IsZero():
				data.SetDateMaturity(data.Date())
			default:
				data.SetDateMaturity(move.Date())
			}

			ok = !(journal.TypeControls().IsNotEmpty() || journal.AccountControls().IsNotEmpty())
			if journal.TypeControls().IsNotEmpty() {
				typ := account.UserType()
				for _, t := range journal.TypeControls().Records() {
					if t.Equals(typ) {
						ok = true
						break
					}
				}
			}
			if journal.AccountControls().IsNotEmpty() {
				id := data.Account().ID()
				for _, i := range journal.AccountControls().Ids() {
					if i == id {
						ok = true
						break
					}
				}
			}

			// Automatically convert in the account's secondary currency if there is one and
			// the provided values were not already multi-currency
			if account.Currency().IsNotEmpty() && !data.HasAmountCurrency() && !account.Currency().Equals(account.Company().Currency()) {
				data.SetCurrency(account.Currency())
				if ctx.GetString("skip_full_reconcile_check") == "amount_currency_excluded" {
					data.SetAmountCurrency(0.0)
				} else {
					context := types.NewContext()
					if data.HasDate() {
						context = context.WithKey("date", data.Date())
					}
					data.SetAmountCurrency(account.Company().Currency().WithNewContext(context).Compute(amount, account.Currency(), true))
				}
			}

			if !ok {
				panic(rs.T(`You cannot use this general account in this journal, check the tab 'Entry Controls' on the related journal.`))
			}

			// Create tax lines
			if ctx.GetBool("apply_taxes") && data.Taxes().IsNotEmpty() {
				var totalExcl float64
				var taxes []accounttypes.AppliedTaxData

				_, totalExcl, _, taxes = data.Taxes().WithContext("round", true).ComputeAll(amount, data.Currency(), 1, data.Product(), data.Partner())
				// Adjust line amount if any tax is price_include
				if math.Abs(totalExcl) < math.Abs(amount) {
					if data.Debit() != 0.0 {
						data.SetDebit(totalExcl)
					}
					if data.Credit() != 0.0 {
						data.SetCredit(-totalExcl)
					}
					if data.AmountCurrency() != 0.0 {
						data.SetAmountCurrency(data.Currency().Round(data.AmountCurrency() * (totalExcl / amount)))
					}
				}

				// Create tax lines
				for _, taxData := range taxes {
					if taxData.Amount == 0.0 {
						continue
					}

					var tax m.AccountTaxSet
					var accountID int64
					var account m.AccountAccountSet
					var temp m.AccountMoveLineData
					var bank m.AccountBankStatementSet

					tax = h.AccountTax().BrowseOne(rs.Env(), taxData.ID)
					if amount > 0 {
						accountID = taxData.AccountID
					} else {
						accountID = taxData.RefundAccountID
					}
					if accountID == 0 {
						account = data.Account()
					} else {
						account = h.AccountAccount().BrowseOne(rs.Env(), accountID)
					}

					temp = h.AccountMoveLine().NewData().
						SetAccount(account).
						SetName(data.Name() + " " + taxData.Name).
						SetTaxLine(tax).
						SetMove(data.Move()).
						SetPartner(data.Partner()).
						SetStatement(data.Statement()).
						SetDebit(0.0).
						SetCredit(0.0)
					if tax.Analytic() {
						temp.SetAnalyticAccount(data.AnalyticAccount())
					}
					if taxData.Amount > 0 {
						temp.SetDebit(taxData.Amount)
					} else if taxData.Amount < 0 {
						temp.SetCredit(-taxData.Amount)
					}

					bank = data.Statement()
					if !bank.Currency().Equals(bank.Company().Currency()) {
						var context *types.Context

						context = types.NewContext()
						if data.HasDate() {
							context = context.WithKey("date", data.Date())
						} else if data.HasDateMaturity() {
							context = context.WithKey("date", data.DateMaturity())
						}
						temp.SetCurrency(bank.Currency())
						temp.SetAmountCurrency(bank.Company().Currency().WithNewContext(ctx).Compute(taxData.Amount, bank.Currency(), true))
					}
					taxLinesData = append(taxLinesData, temp)
				}
			}
			newLine = rs.Super().Create(data)
			for _, line := range taxLinesData {
				// TODO: remove .with_context(context) once this context nonsense is solved
				rs.WithNewContext(ctx).Create(line)
			}
			if !ctx.HasKey("check_move_validity") || ctx.GetBool("check_move_validity") {
				move.WithNewContext(ctx).PostValidate()
			}
			return newLine
		})

	h.AccountMoveLine().Methods().Unlink().Extend("",
		func(rs m.AccountMoveLineSet) int64 {
			var moves m.AccountMoveSet
			var result int64

			rs.UpdateCheck()
			moves = h.AccountMove().NewSet(rs.Env())
			for _, line := range rs.Records() {
				moves = moves.Union(line.Move())
			}
			result = rs.Super().Unlink()
			if rs.Env().Context().GetBool("check_move_validity") && moves.IsNotEmpty() {
				moves.PostValidate()
			}
			return result
		})

	h.AccountMoveLine().Methods().Write().Extend("",
		func(rs m.AccountMoveLineSet, data m.AccountMoveLineData) bool {
			if data.Account().Deprecated() {
				panic(rs.T(`You cannot use deprecated account.`))
			}
			if data.HasAccount() || data.HasJournal() || data.HasDate() || data.HasMove() || data.HasDebit() || data.HasCredit() {
				rs.UpdateCheck()
			}
			if !rs.Env().Context().GetBool("allow_amount_currency") && (data.HasAmountCurrency() || data.HasCurrency()) {
				//hackish workaround to write the amount_currency when assigning a payment to an invoice through the 'add' button
				//this is needed to compute the correct amount_residual_currency and potentially create an exchange difference entry
				rs.UpdateCheck()
			}
			//when we set the expected payment date, log a note on the invoice_id related (if any)
			/*
				if vals.get('expected_pay_date') and self.invoice_id: //tovalid expected_pay_date field missing in data
					msg = _('New expected payment date: ') + vals['expected_pay_date'] + '.\n' + vals.get('internal_note', '')
					self.invoice_id.message_post(body=msg) #TODO: check it is an internal note (not a regular email)!
			*/

			// when making a reconciliation on an existing liquidity journal item, mark the payment as reconciled
			for _, rec := range rs.Records() {
				if data.HasStatement() && rec.Payment().IsNotEmpty() {
					// In case of an internal transfer, there are 2 liquidity move lines to match with a bank statement
					all := true
					for _, line := range rec.Payment().MoveLines().Filtered(func(r m.AccountMoveLineSet) bool { return !r.Equals(rec) && r.Account().InternalType() == "liquidity" }).Records() {
						if line.Statement().IsEmpty() {
							all = false
						}
					}
					if all {
						rec.Payment().SetState("reconciled")
					}
				}
			}

			result := rs.Super().Write(data)
			if rs.Env().Context().GetBool("check_move_validity") || !rs.Env().Context().HasKey("check_move_validity") {
				moves := h.AccountMove().NewSet(rs.Env())
				for _, line := range rs.Records() {
					moves = moves.Union(line.Move())
				}
				moves.PostValidate()
			}
			return result
		})

	h.AccountMoveLine().Methods().UpdateCheck().DeclareMethod(
		`UpdateCheck Raise Warning to cause rollback if the move is posted, some entries are reconciled or the move is older than the lock date`,
		func(rs m.AccountMoveLineSet) bool {
			moves := h.AccountMove().NewSet(rs.Env())
			for _, line := range rs.Records() {
				errMsg := rs.T(`Move name (id): %s (%d)`, line.Move().Name(), line.Move().ID())
				if line.Move().State() != "draft" {
					panic(rs.T(`You cannot do this modification on a posted journal entry, you can just change some non legal fields.
								You must revert the journal entry to cancel it.\n%s.`, errMsg))
				}
				if line.Reconciled() && !(line.Debit() == 0.0 && line.Credit() == 0.0) {
					panic(rs.T(`You cannot do this modification on a reconciled entry.
								You can just change some non legal fields or you must unreconcile first.\n%s.`, errMsg))
				}
				moves = moves.Union(line.Move())
			}
			moves.CheckLockDate()
			return true
		})

	h.AccountMoveLine().Methods().NameGet().Extend("",
		func(rs m.AccountMoveLineSet) string {
			/*def name_get(self):
			for line in self:
				if line.ref:
					result.append((line.id, (line.move_id.name or '') + '(' + line.ref + ')'))
				else:
					result.append((line.id, line.move_id.name))
			return result
			*/
			return rs.Super().NameGet()
		})

	h.AccountMoveLine().Methods().ComputeAmountFields().DeclareMethod(
		`ComputeAmountFields Helper function to compute value for fields debit/credit/amount_currency based on an amount and the currencies given in parameter`,
		func(rs m.AccountMoveLineSet, amount float64, srcCurrency, companyCurrency, invoiceCurrency m.CurrencySet) (float64, float64, float64, m.CurrencySet) {
			var amountCurrency float64
			var currency m.CurrencySet
			var debit float64
			var credit float64

			if srcCurrency.IsNotEmpty() && !srcCurrency.Equals(companyCurrency) {
				amountCurrency = amount
				amount = srcCurrency.WithNewContext(rs.Env().Context()).Compute(amount, companyCurrency, true)
				currency = srcCurrency
			}
			if amount > 0 {
				debit = amount
			} else if amount < 0 {
				credit = -amount
			}
			if invoiceCurrency.IsNotEmpty() && !invoiceCurrency.Equals(companyCurrency) && amountCurrency == 0.0 {
				amountCurrency = srcCurrency.WithNewContext(rs.Env().Context()).Compute(amount, invoiceCurrency, true)
				currency = invoiceCurrency
			}
			return debit, credit, amountCurrency, currency
		})

	h.AccountMoveLine().Methods().CreateAnalyticLines().DeclareMethod(
		`CreateAnalyticLines Create analytic items upon validation of an account.move.line having an analytic account. This
			      method first remove any existing analytic item related to the line before creating any new one.`,
		func(rs m.AccountMoveLineSet) {
			for _, line := range rs.Records() {
				line.AnalyticLines().Unlink()
			}
			for _, line := range rs.Records() {
				if line.AnalyticAccount().IsNotEmpty() {
					data := line.PrepareAnalyticLine()
					h.AccountAnalyticLine().Create(rs.Env(), data)
				}
			}
		})

	h.AccountMoveLine().Methods().PrepareAnalyticLine().DeclareMethod(
		`PrepareAnalyticLin Prepare the values used to create() an account.analytic.line upon validation of an account.move.line having
			      an analytic account. This method is intended to be extended in other modules.e`,
		func(rs m.AccountMoveLineSet) m.AccountAnalyticLineData {
			var amount float64
			var data m.AccountAnalyticLineData
			var date dates.Date

			amount = rs.Credit() - rs.Debit()
			data = h.AccountAnalyticLine().NewData().
				SetName(rs.Name()).
				SetDate(rs.Date()).
				SetAccount(rs.AnalyticAccount()).
				SetTags(rs.AnalyticTags()).
				SetUnitAmount(rs.Quantity()).
				SetProduct(rs.Product()).
				SetAmount(amount).
				SetProductUom(rs.ProductUom()).
				SetGeneralAccount(rs.Account()).
				SetRef(rs.Ref()).
				SetMove(rs).
				SetUser(rs.Invoice().User())
			date = rs.Date()
			if date.IsZero() {
				date = dates.Today()
			}
			if curr := rs.AnalyticAccount().Currency(); curr.IsNotEmpty() {
				data.SetAmount(rs.CompanyCurrency().WithContext("date", date).Compute(amount, rs.AnalyticAccount().Currency(), true))
			}
			return data
		})

	h.AccountMoveLine().Methods().QueryGet().DeclareMethod(
		`QueryGet`,
		func(rs m.AccountMoveLineSet, condition q.AccountMoveLineCondition) (string, []interface{}) {
			context := rs.Env().Context()
			dateField := q.AccountMoveLine().Date()
			if context.GetBool("aged_balance") {
				dateField = q.AccountMoveLine().DateMaturity()
			}
			if val := context.GetDate("date_to"); !val.IsZero() {
				condition = condition.AndCond(dateField.LowerOrEqual(val))
			}
			if val := context.GetDate("date_from"); !val.IsZero() {
				switch {
				case !context.GetBool("strict_range"):
					condition = condition.AndCond(dateField.GreaterOrEqual(val).Or().
						AccountFilteredOn(
							q.AccountAccount().UserTypeFilteredOn(
								q.AccountAccountType().IncludeInitialBalance().Equals(true))))
				case context.GetBool("initial_bal"):
					condition = condition.AndCond(dateField.Lower(val))
				default:
					condition = condition.AndCond(dateField.GreaterOrEqual(val))
				}
			}
			if val := context.GetIntegerSlice("journal_ids"); len(val) > 0 {
				condition = condition.AndCond(q.AccountMoveLine().JournalFilteredOn(q.AccountJournal().ID().In(val)))
			}
			if val := context.GetString("state"); val != "" && strings.ToLower(val) != "all" {
				condition = condition.AndCond(q.AccountMoveLine().MoveFilteredOn(q.AccountMove().State().Equals(val)))
			}
			if val := context.GetInteger("company_id"); val != 0 {
				condition = condition.AndCond(q.AccountMoveLine().CompanyFilteredOn(q.Company().ID().Equals(val)))
			}
			if context.HasKey("company_ids") {
				condition = condition.AndCond(q.AccountMoveLine().CompanyFilteredOn(q.Company().ID().In(context.GetIntegerSlice("company_ids"))))
			}
			if val := context.GetDateTime("reconcile_date"); !val.IsZero() {
				condition = condition.AndCond(q.AccountMoveLine().Reconciled().Equals(false).
					Or().MatchedDebitsFilteredOn(q.AccountPartialReconcile().CreateDate().Greater(val)).
					Or().MatchedCreditsFilteredOn(q.AccountPartialReconcile().CreateDate().Greater(val)))
			}
			if val := context.GetIntegerSlice("account_tag_ids"); len(val) > 0 {
				condition = condition.AndCond(q.AccountMoveLine().AccountFilteredOn(q.AccountAccount().TagsFilteredOn(q.AccountAccountTag().ID().In(val))))
			}
			if val := context.GetIntegerSlice("analytic_tag_ids"); len(val) > 0 {
				condition = condition.AndCond(q.AccountMoveLine().AnalyticTagsFilteredOn(q.AccountAnalyticTag().ID().In(val)).
					Or().AnalyticAccountFilteredOn(q.AccountAnalyticAccount().TagsFilteredOn(q.AccountAnalyticTag().ID().In(val))))
			}
			if val := context.GetIntegerSlice("analytic_account_ids"); len(val) > 0 {
				condition = condition.AndCond(q.AccountMoveLine().AnalyticAccountFilteredOn(q.AccountAnalyticAccount().ID().In(val)))
			}
			if !condition.IsEmpty() {
				// FIXME
				//return rs.SqlFromCondition(condition.Condition)
			}
			return "", []interface{}{}
		})

	h.AccountMoveLine().Methods().OpenReconcileView().DeclareMethod(
		`OpenReconcileView`,
		func(rs m.AccountMoveLineSet) *actions.Action {
			/*def open_reconcile_view(self):
			[action] = self.env.ref('account.action_account_moves_all_a').read() //tovalid
			ids = []
			for aml in self:
				if aml.account_id.reconcile:
					ids.extend([r.debit_move_id.id for r in aml.matched_debit_ids] if aml.credit > 0 else [r.credit_move_id.id for r in aml.matched_credit_ids])
					ids.append(aml.id)
			action['domain'] = [('id', 'in', ids)]
			return action
			*/
			return new(actions.Action)
		})

	h.AccountPartialReconcile().DeclareModel()

	h.AccountPartialReconcile().AddFields(map[string]models.FieldDefinition{
		"DebitMove": models.Many2OneField{
			RelationModel: h.AccountMoveLine(),
			Index:         true,
			Required:      true},
		"CreditMove": models.Many2OneField{
			RelationModel: h.AccountMoveLine(),
			Index:         true,
			Required:      true},
		"Amount": models.FloatField{
			Help: "Amount concerned by this matching. Assumed to be always positive"},
		"AmountCurrency": models.FloatField{
			String: "Amount in Currency"},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency()},
		"CompanyCurrency": models.Many2OneField{
			RelationModel: h.Currency(),
			Related:       "Company.Currency",
			ReadOnly:      true,
			Help:          "Utility field to express amount currency"},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Related:       "DebitMove.Company"},
		"FullReconcile": models.Many2OneField{
			RelationModel: h.AccountFullReconcile(),
			NoCopy:        true},
	})

	h.AccountPartialReconcile().Methods().CreateExchangeRateEntry().DeclareMethod(
		`CreateExchangeRateEntry Automatically create a journal entry to book the exchange rate difference.
			      That new journal entry is made in the company 'currency_exchange_journal_id' and one of its journal
			      items is matched with the other lines to balance the full reconciliation.`,
		func(rs m.AccountPartialReconcileSet, amlToFix m.AccountMoveLineSet, amountDiff float64, diffInCurrency float64,
			currency m.CurrencySet, moveDate dates.Date) (m.AccountMoveLineSet, m.AccountPartialReconcileSet) {
			var lineToReconcile m.AccountMoveLineSet
			var partialRec m.AccountPartialReconcileSet

			for _, rec := range rs.Records() {
				if rec.Company().CurrencyExchangeJournal().IsEmpty() {
					panic(rs.T(`You should configure the 'Exchange Rate Journal' in the accounting settings, to manage automatically the booking of accounting entries related to differences between exchange rates.`))
				}
				if rec.Company().IncomeCurrencyExchangeAccount().IsEmpty() {
					panic(rs.T(`You should configure the 'Gain Exchange Rate Account' in the accounting settings, to manage automatically the booking of accounting entries related to differences between exchange rates.`))
				}
				if rec.Company().ExpenseCurrencyExchangeAccount().IsEmpty() {
					panic(rs.T(`You should configure the 'Loss Exchange Rate Account' in the accounting settings, to manage automatically the booking of accounting entries related to differences between exchange rates.`))
				}

				var move m.AccountMoveSet
				var amlData m.AccountMoveLineData
				var moveData m.AccountMoveData
				var partialRecData m.AccountPartialReconcileData
				var lineToReconcileData m.AccountMoveLineData

				moveData = h.AccountMove().NewData().SetJournal(rec.Company().CurrencyExchangeJournal())
				// The move date should be the maximum date between payment and invoice (in case
				// of payment in advance). However, we should make sure the move date is not
				// recorded after the end of year closing.
				if moveDate.Greater(rec.Company().FiscalyearLockDate()) {
					moveData.SetDate(moveDate)
				}
				move = h.AccountMove().Create(rs.Env(), moveData)
				amountDiff = rec.Company().Currency().Round(amountDiff)
				diffInCurrency = currency.Round(diffInCurrency)

				lineToReconcileData = h.AccountMoveLine().NewData().
					SetName(rs.T(`Currency exchange rate difference`)).
					SetDebit(0.0).
					SetCredit(0.0).
					SetAccount(rec.DebitMove().Account()).
					SetMove(move).
					SetCurrency(currency).
					SetAmountCurrency(-diffInCurrency).
					SetPartner(rec.DebitMove().Partner())
				if amountDiff < 0 {
					lineToReconcileData.SetDebit(-amountDiff)
				} else if amountDiff > 0 {
					lineToReconcileData.SetCredit(amountDiff)
				}
				lineToReconcile = h.AccountMoveLine().NewSet(rs.Env()).WithContext("check_move_validity", false).Create(lineToReconcileData)

				amlData = h.AccountMoveLine().NewData().
					SetName(rs.T(`Currency exchange rate difference`)).
					SetDebit(0.0).
					SetCredit(0.0).
					SetAccount(rec.Company().CurrencyExchangeJournal().DefaultCreditAccount()).
					SetMove(move).
					SetCurrency(currency).
					SetAmountCurrency(diffInCurrency).
					SetPartner(rec.DebitMove().Partner())
				if amountDiff > 0 {
					amlData.SetDebit(amountDiff)
					amlData.SetAccount(rec.Company().CurrencyExchangeJournal().DefaultDebitAccount())
				} else if amountDiff < 0 {
					amlData.SetCredit(-amountDiff)
				}
				h.AccountMoveLine().Create(rs.Env(), amlData)

				for _, aml := range amlToFix.Records() {
					partialRecData = h.AccountPartialReconcile().NewData().
						SetDebitMove(aml).
						SetCreditMove(aml).
						SetAmount(math.Abs(aml.AmountResidual())).
						SetAmountCurrency(math.Abs(aml.AmountResidualCurrency())).
						SetCurrency(currency)
					if aml.Credit() != 0.0 {
						partialRecData.SetDebitMove(lineToReconcile)
					}
					if aml.Debit() != 0.0 {
						partialRecData.SetCreditMove(lineToReconcile)
					}
				}

				move.Post()
			}
			return lineToReconcile, partialRec
		})

	h.AccountPartialReconcile().Methods().FixMultipleExchangeRatesDiff().DeclareMethod(
		`FixMultipleExchangeRatesDiff`,
		func(rs m.AccountPartialReconcileSet, amlsToFix m.AccountMoveLineSet, amountDiff float64, diffInCurrency float64,
			currency m.CurrencySet, move m.AccountMoveSet) (m.AccountMoveLineSet, m.AccountPartialReconcileSet) {

			rs.EnsureOne()

			var moveLines m.AccountMoveLineSet
			var accountPayableLine m.AccountMoveLineSet
			var partialRec m.AccountPartialReconcileSet
			var partialReconciles m.AccountPartialReconcileSet
			var accountPayableLineData m.AccountMoveLineData
			var moveLineData m.AccountMoveLineData
			var partialRecData m.AccountPartialReconcileData

			moveLines = h.AccountMoveLine().NewSet(rs.Env()).WithContext("check_move_validity", false)
			partialReconciles = rs.WithContext("skip_full_reconcile_check", true)
			amountDiff = rs.Company().Currency().Round(amountDiff)

			for _, aml := range amlsToFix.Records() {
				accountPayableLineData = h.AccountMoveLine().NewData().
					SetName(rs.T(`Currency exchange rate difference`)).
					SetDebit(0.0).
					SetCredit(0.0).
					SetAccount(rs.DebitMove().Account()).
					SetMove(move).
					SetCurrency(currency).
					SetAmountCurrency(-aml.AmountResidualCurrency()).
					SetPartner(rs.DebitMove().Partner())
				moveLineData = h.AccountMoveLine().NewData().
					SetName(rs.T(`Currency exchange rate difference`)).
					SetDebit(0.0).
					SetCredit(0.0).
					SetAccount(rs.Company().CurrencyExchangeJournal().DefaultCreditAccount()).
					SetMove(move).
					SetCurrency(currency).
					SetAmountCurrency(aml.AmountResidualCurrency()).
					SetPartner(rs.DebitMove().Partner())
				if amountDiff > 0 {
					accountPayableLineData.SetCredit(aml.AmountResidual())
					moveLineData.SetDebit(aml.AmountResidual())
					moveLineData.SetAccount(rs.Company().CurrencyExchangeJournal().DefaultDebitAccount())
				} else if amountDiff < 0 {
					accountPayableLineData.SetDebit(-aml.AmountResidual())
					moveLineData.SetCredit(-aml.AmountResidual())
				}
				accountPayableLine = moveLines.Create(accountPayableLineData)
				moveLines.Create(moveLineData)

				partialRecData = h.AccountPartialReconcile().NewData().
					SetDebitMove(aml).
					SetCreditMove(aml).
					SetAmount(math.Abs(aml.AmountResidual())).
					SetAmountCurrency(math.Abs(aml.AmountResidualCurrency())).
					SetCurrency(currency)
				if aml.Credit() != 0.0 {
					partialRecData.SetDebitMove(accountPayableLine)
				}
				if aml.Debit() != 0.0 {
					partialRecData.SetCreditMove(accountPayableLine)
				}
				partialRec = rs.Super().Create(partialRecData)

				moveLines = moveLines.Union(accountPayableLine)
				partialReconciles = partialReconciles.Union(partialRec)
			}
			partialReconciles.ComputePartialLines()
			return moveLines, partialReconciles
		})

	h.AccountPartialReconcile().Methods().ComputePartialLines().DeclareMethod(
		`ComputePartialLines`,
		func(rs m.AccountPartialReconcileSet) {
			if rs.Env().Context().GetBool("skip_full_reconcile_check") {
				// when running the manual reconciliation wizard, don't check the partials separately for full
				// reconciliation or exchange rate because it is handled manually after the whole processing
				return
			}

			var totalDebit float64
			var totalCredit float64
			var totalAmountCurrency float64
			var digitsRoundingPrecision float64
			var maxDate dates.Date
			var currency m.CurrencySet
			var exchangeMove m.AccountMoveSet
			var amlSet m.AccountMoveLineSet
			var amlToBalance m.AccountMoveLineSet
			var exchangePartialRec m.AccountPartialReconcileSet
			var partialRec m.AccountPartialReconcileSet
			var partialRecSet m.AccountPartialReconcileSet

			// check if the reconciliation is full
			// first, gather all journal items involved in the reconciliation just created
			partialRecSet = rs.Sorted(func(rs1, rs2 m.AccountPartialReconcileSet) bool {
				return rs1.ID() < rs2.ID()
			})
			amlSet = h.AccountMoveLine().NewSet(rs.Env())
			amlToBalance = h.AccountMoveLine().NewSet(rs.Env())
			currency = partialRecSet.Currency()

			// make sure that all partial reconciliations share the same secondary currency otherwise it's not
			// possible to compute the exchange difference entry and it has to be done manually.
			for _, partialRec = range partialRecSet.Records() {
				if !partialRec.Currency().Equals(currency) {
					// no exchange rate entry will be created
					currency = h.Currency().NewSet(rs.Env())
				}
				for _, aml := range partialRec.DebitMove().Union(partialRec.CreditMove()).Records() {
					if aml.Intersect(amlSet).Len() == 0 {
						if aml.AmountResidual() != 0.0 || aml.AmountResidualCurrency() != 0.0 {
							amlToBalance = amlToBalance.Union(aml)
						}
						if aml.Date().Greater(maxDate) {
							maxDate = aml.Date()
						}
						totalDebit += aml.Debit()
						totalCredit += aml.Credit()
						amlSet = amlSet.Union(aml)
						if aml.Currency().IsNotEmpty() && aml.Currency().Equals(currency) {
							totalAmountCurrency += aml.AmountCurrency()
						} else if partialRec.Currency().IsNotEmpty() && partialRec.Currency().Equals(currency) {
							// if the aml has no secondary currency but is reconciled with other journal item(s) in secondary currency, the amount
							// currency is recorded on the partial rec and in order to check if the reconciliation is total, we need to convert the
							// aml.balance in that foreign currency
							totalAmountCurrency += aml.Company().Currency().WithContext("date", aml.Date()).Compute(aml.Balance(), partialRec.Currency(), true)
						}
					}
					for _, id := range aml.MatchedDebits().Union(aml.MatchedCredits()).Ids() {
						partialRecSet.Records()[id] = h.AccountPartialReconcile().NewSet(rs.Env()) //tovalid wut?
					}
				}
			}
			// then, if the total debit and credit are equal, or the total amount in currency is 0, the reconciliation is full
			digitsRoundingPrecision = amlSet.Company().Currency().Rounding()
			if !((currency.IsNotEmpty() && nbutils.IsZero(totalAmountCurrency, currency.Rounding())) || nbutils.Compare(totalDebit, totalCredit, digitsRoundingPrecision) == 0.0) {
				return
			}
			if currency.IsNotEmpty() && amlToBalance.IsNotEmpty() {
				exchangeMove = h.AccountMove().Create(rs.Env(), h.AccountFullReconcile().NewSet(rs.Env()).PrepareExchangeDiffMove(maxDate, amlToBalance.Company()))
				// eventually create a journal entry to book the difference due to foreign currency's exchange rate that fluctuates
				rateDiffAmls, rateDiffPartialRecs := partialRec.FixMultipleExchangeRatesDiff(amlToBalance, totalDebit-totalCredit, totalAmountCurrency, currency, exchangeMove)
				amlSet = amlSet.Union(rateDiffAmls)
				partialRecSet = partialRecSet.Union(rateDiffPartialRecs)
				exchangeMove.Post()
				exchangePartialRec = rateDiffPartialRecs.Records()[rateDiffPartialRecs.Len()-1]
			}
			// mark the reference of the full reconciliation on the partial ones and on the entries
			h.AccountFullReconcile().NewSet(rs.Env()).WithContext("check_move_validity", false).Create(
				h.AccountFullReconcile().NewData().
					SetPartialReconciles(partialRecSet).
					SetReconciledLines(amlSet).
					SetExchangeMove(exchangeMove).
					SetExchangePartialRec(exchangePartialRec))
		})

	h.AccountPartialReconcile().Methods().Create().Extend(
		"",
		func(rs m.AccountPartialReconcileSet, data m.AccountPartialReconcileData) m.AccountPartialReconcileSet {
			res := rs.Super().Create(data)
			res.ComputePartialLines()
			return res
		})

	h.AccountPartialReconcile().Methods().Unlink().Extend(
		"When removing a partial reconciliation, also unlink its full reconciliation if it exists",
		func(rs m.AccountPartialReconcileSet) int64 {
			var res int64
			var toUnlink m.AccountPartialReconcileSet
			var fullToUnlink m.AccountFullReconcileSet

			fullToUnlink = h.AccountFullReconcile().NewSet(rs.Env())
			toUnlink = rs.Copy(nil)
			if !rs.Env().Context().HasKey("full_rec_lookup") || rs.Env().Context().GetBool("full_rec_lookup") {
				for _, rec := range rs.Records() {
					// exclude partial reconciliations related to an exchange rate entry, because the unlink of the full reconciliation will already do it
					if h.AccountFullReconcile().Search(rs.Env(), q.AccountFullReconcile().ExchangePartialRecFilteredOn(q.AccountPartialReconcile().ID().Equals(rec.ID()))).IsNotEmpty() {
						toUnlink = toUnlink.Subtract(rec)
					}
					// without the deleted partial reconciliations, the full reconciliation won't be full anymore
					if rec.FullReconcile().IsNotEmpty() {
						fullToUnlink = fullToUnlink.Union(rec.FullReconcile())
					}
				}
			}
			if toUnlink.IsNotEmpty() {
				res = toUnlink.Super().Unlink()
			}
			if fullToUnlink.IsNotEmpty() {
				fullToUnlink.Unlink()
			}
			return res
		})

	h.AccountFullReconcile().DeclareModel()

	h.AccountFullReconcile().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:   "Number",
			Required: true,
			NoCopy:   true,
			Default: func(env models.Environment) interface{} {
				return h.Sequence().NewSet(env).NextByCode("account.reconcile")
			}},
		"PartialReconciles": models.One2ManyField{
			String:        "Reconciliation Parts",
			RelationModel: h.AccountPartialReconcile(),
			ReverseFK:     "FullReconcile",
			JSON:          "partial_reconcile_ids"},
		"ReconciledLines": models.One2ManyField{
			String:        "Matched Journal Items",
			RelationModel: h.AccountMoveLine(),
			ReverseFK:     "FullReconcile",
			JSON:          "reconciled_line_ids"},
		"ExchangeMove": models.Many2OneField{
			RelationModel: h.AccountMove()},
		"ExchangePartialRec": models.Many2OneField{
			RelationModel: h.AccountPartialReconcile()},
	})

	h.AccountFullReconcile().Methods().Unlink().Extend(
		`When removing a full reconciliation, we need to revert the eventual journal entries we created to book the
				fluctuation of the foreign currency's exchange rate.
				We need also to reconcile together the origin currency difference line and its reversal in order to completly
				cancel the currency difference entry on the partner account (otherwise it will still appear on the aged balance
				for example).`,
		func(rs m.AccountFullReconcileSet) int64 {
			for _, rec := range rs.Records() {
				if rec.ExchangeMove().IsEmpty() {
					continue
				}
				// reverse the exchange rate entry
				// reconciliation of the exchange move and its reversal is handled in reverse_moves
				rec.ExchangeMove().ReverseMoves(dates.Date{}, h.AccountJournal().NewSet(rs.Env()))
				rec.SetExchangeMove(h.AccountMove().NewSet(rs.Env()))
			}
			return rs.Super().Unlink()
		})

	h.AccountFullReconcile().Methods().PrepareExchangeDiffMove().DeclareMethod(
		`PrepareExchangeDiffMove`,
		func(rs m.AccountFullReconcileSet, moveDate dates.Date, company m.CompanySet) m.AccountMoveData {
			if company.CurrencyExchangeJournal().IsEmpty() {
				panic(rs.T(`You should configure the 'Exchange Rate Journal' in the accounting settings, to manage automatically the booking of accounting entries related to differences between exchange rates.`))
			}
			if company.IncomeCurrencyExchangeAccount().IsEmpty() {
				panic(rs.T(`You should configure the 'Gain Exchange Rate Account' in the accounting settings, to manage automatically the booking of accounting entries related to differences between exchange rates.`))
			}
			if company.ExpenseCurrencyExchangeAccount().IsEmpty() {
				panic(rs.T(`You should configure the 'Loss Exchange Rate Account' in the accounting settings, to manage automatically the booking of accounting entries related to differences between exchange rates.`))
			}
			res := h.AccountMove().NewData().
				SetJournal(company.CurrencyExchangeJournal())
			if moveDate.Greater(company.FiscalyearLockDate()) {
				res.SetDate(moveDate)
			}
			return res
		})

}
