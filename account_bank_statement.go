// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"fmt"
	"math"

	"github.com/hexya-addons/account/accounttypes"
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
	"github.com/hexya-erp/hexya/src/views"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	"github.com/jmoiron/sqlx"
)

func init() {

	h.AccountCashboxLine().DeclareModel()
	h.AccountCashboxLine().SetDefaultOrder("CoinValue")

	h.AccountCashboxLine().AddFields(map[string]models.FieldDefinition{
		"CoinValue": models.FloatField{
			String:   "Coin/Bill Value",
			Required: true},
		"Number": models.IntegerField{
			String: "Number of Coins/Bills",
			Help:   "Opening Unit Numbers"},
		"Subtotal": models.FloatField{
			Compute:  h.AccountCashboxLine().Methods().SubTotal(),
			ReadOnly: true},
		"Cashbox": models.Many2OneField{
			String:        "Cashbox",
			RelationModel: h.AccountBankStatementCashbox()},
	})

	h.AccountCashboxLine().Methods().SubTotal().DeclareMethod(
		`Calculates Sub total`,
		func(rs m.AccountCashboxLineSet) m.AccountCashboxLineData {
			value := rs.CoinValue() * float64(rs.Number())
			res := h.AccountCashboxLine().NewData()

			if rs.Subtotal() != value {
				res.SetSubtotal(value)
			}
			return res
		})

	h.AccountBankStatementCashbox().DeclareModel()

	h.AccountBankStatementCashbox().AddFields(map[string]models.FieldDefinition{
		"CashboxLines": models.One2ManyField{
			RelationModel: h.AccountCashboxLine(),
			ReverseFK:     "Cashbox",
			JSON:          "cashbox_lines_ids"},
	})

	h.AccountBankStatementCashbox().Methods().Validate().DeclareMethod(
		`Validate`,
		func(rs m.AccountBankStatementCashboxSet) *actions.Action {
			bankStmtId := rs.Env().Context().GetInteger("bank_statement_id")
			if bankStmtId == 0 {
				bankStmtId = rs.Env().Context().GetInteger("active_id")
			}
			bankStmt := h.AccountBankStatement().Browse(rs.Env(), []int64{bankStmtId})
			total := 0.0
			for _, line := range rs.CashboxLines().Records() {
				total += line.Subtotal()
			}
			res := h.AccountBankStatement().NewData()
			if rs.Env().Context().GetString("balance") == "start" {
				//starting balance
				res.SetBalanceStart(total)
				res.SetCashboxStart(rs)
			} else {
				//closing balance
				res.SetBalanceEndReal(total)
				res.SetCashboxEnd(rs)
			}
			bankStmt.Write(res)
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

	h.AccountBankStatementClosebalance().DeclareTransientModel()

	h.AccountBankStatementClosebalance().Methods().Validate().DeclareMethod(
		`Validate`,
		func(rs m.AccountBankStatementClosebalanceSet) *actions.Action {
			id := rs.Env().Context().GetInteger("active_id")
			if id > 0 {
				h.AccountBankStatement().Browse(rs.Env(), []int64{id}).ButtonConfirmBank()
			}
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

	h.AccountBankStatement().DeclareModel()
	h.AccountBankStatement().SetDefaultOrder("Date DESC", "ID DESC")

	h.AccountBankStatement().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String: "Reference",
			/*[ states {'open': [('readonly'] '=' [ False)]}]*/
			//ReadOnly: true, //tovalid those readonly
			NoCopy: true},
		"Reference": models.CharField{
			String: "External Reference",
			/*[ states {'open': [('readonly'] '=' [ False)]}]*/
			//ReadOnly: true,
			NoCopy: true,
			Help: `Used to hold the reference of the external mean that created this statement
(name of imported file, reference of online synchronization...)`},
		"Date": models.DateField{
			Required: true,
			/*[ states {'confirm': [('readonly']*/ /*[ True)]}]*/
			Index:                                 true,
			NoCopy:                                true, Default: func(env models.Environment) interface{} {
				return dates.Today()
			}},
		"DateDone": models.DateTimeField{
			String: "Closed On"},
		"BalanceStart": models.FloatField{
			String: "Starting Balance",
			/*[ states {'confirm': [('readonly'] '=' [ True)]}]*/
			Default: func(env models.Environment) interface{} {
				// Search last bank statement and set current opening balance as closing balance of previous one
				journal := h.AccountJournal().NewSet(env)
				switch {
				case env.Context().HasKey("default_journal_id"):
					journal = h.AccountJournal().Browse(env, []int64{env.Context().GetInteger("default_journal_id")})
				case env.Context().HasKey("journal_id"):
					journal = h.AccountJournal().Browse(env, []int64{env.Context().GetInteger("default_journal_id")})
				}
				return h.AccountBankStatement().NewSet(env).GetOpeningBalance(journal)
			}},
		"BalanceEndReal": models.FloatField{
			String: "Ending Balance",
			/*[ states {'confirm': [('readonly'] '=' [ True)]}]*/},
		"State": models.SelectionField{
			String: "Status",
			Selection: types.Selection{
				"open":    "New",
				"confirm": "Validated"},
			Required: true,
			ReadOnly: true,
			NoCopy:   true,
			Default:  models.DefaultValue("open")},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency(),
			Compute:       h.AccountBankStatement().Methods().ComputeCurrency(),
			Depends:       []string{"Journal"}},
		"Journal": models.Many2OneField{
			String:        "Journal",
			RelationModel: h.AccountJournal(),
			Required:      true,
			/*[ states {'confirm': [('readonly']*/ /*[ True)]}]*/
			OnChange:                              h.AccountBankStatement().Methods().OnchangeJournal(),
			Default: func(env models.Environment) interface{} {
				journalType := env.Context().GetString("journal_type")
				company := h.Company().NewSet(env).CompanyDefaultGet()
				if journalType != "" {
					return h.AccountJournal().Search(env,
						q.AccountJournal().Type().Equals(journalType).And().Company().Equals(company)).Limit(1)
				}
				return h.AccountJournal().NewSet(env)
			}},
		"JournalType": models.SelectionField{
			Related: "Journal.Type",
			Help:    "Technical field used for usability purposes"},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Related:       "Journal.Company",
			ReadOnly:      true,
			Default: func(env models.Environment) interface{} {
				return h.Company().NewSet(env).CompanyDefaultGet()
			}},
		"TotalEntryEncoding": models.FloatField{
			String:  "Transactions Subtotal",
			Compute: h.AccountBankStatement().Methods().EndBalance(),
			Stored:  true,
			Depends: []string{"Lines", "BalanceStart", "Lines.Amount", "BalanceEndReal"},
			Help:    "Total of transaction lines."},
		"BalanceEnd": models.FloatField{
			String:  "Computed Balance",
			Compute: h.AccountBankStatement().Methods().EndBalance(),
			Stored:  true,
			Depends: []string{"Lines", "BalanceStart", "Lines.Amount", "BalanceEndReal"},
			Help:    "Balance as calculated based on Opening Balance and transaction lines"},
		"Difference": models.FloatField{
			Compute: h.AccountBankStatement().Methods().EndBalance(),
			Depends: []string{"Lines", "BalanceStart", "Lines.Amount", "BalanceEndReal"},
			Stored:  true,
			Help:    "Difference between the computed ending balance and the specified ending balance."},
		"Lines": models.One2ManyField{
			String:        "Statement Line",
			RelationModel: h.AccountBankStatementLine(),
			ReverseFK:     "Statement",
			JSON:          "line_ids",
			/*[ states {'confirm': [('readonly']*/ /*[ True)]}]*/
			Copy:                                  true},
		"MoveLines": models.One2ManyField{
			String:        "Entry Lines",
			RelationModel: h.AccountMoveLine(),
			ReverseFK:     "Statement",
			JSON:          "move_line_ids",
			/*[ states {'confirm': [('readonly']*/ /*[ True)]}]*/},
		"AllLinesReconciled": models.BooleanField{
			Compute: h.AccountBankStatement().Methods().CheckLinesReconciled(),
			Depends: []string{"Lines.JournalEntries"}},
		"User": models.Many2OneField{
			String:        "Responsible",
			RelationModel: h.User(),
			Required:      false,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser()
			}},
		"CashboxStart": models.Many2OneField{
			String:        "Starting Cashbox",
			RelationModel: h.AccountBankStatementCashbox()},
		"CashboxEnd": models.Many2OneField{
			String:        "Ending Cashbox",
			RelationModel: h.AccountBankStatementCashbox()},
		"IsDifferenceZero": models.BooleanField{
			String:  "Is Zero",
			Compute: h.AccountBankStatement().Methods().ComputeIsDifferenceZero(),
			Help:    "Check if difference is zero."},
	})

	h.AccountBankStatement().Methods().EndBalance().DeclareMethod(
		`EndBalance`,
		func(rs m.AccountBankStatementSet) m.AccountBankStatementData {
			res := h.AccountBankStatement().NewData()
			value := 0.0
			for _, line := range rs.Lines().Records() {
				value += line.Amount()
			}
			if rs.TotalEntryEncoding() != value {
				res.SetTotalEntryEncoding(value)
				res.SetBalanceEnd(rs.BalanceStart() + res.TotalEntryEncoding())
				res.SetDifference(rs.BalanceEndReal() - res.BalanceEnd())
			}
			return res
		})

	h.AccountBankStatement().Methods().ComputeIsDifferenceZero().DeclareMethod(
		`ComputeIsDifferenceZero`,
		func(rs m.AccountBankStatementSet) m.AccountBankStatementData {
			res := h.AccountBankStatement().NewData()
			value := nbutils.IsZero(res.Difference(), nbutils.Digits{16, int8(res.Currency().DecimalPlaces())}.ToPrecision())
			if rs.IsDifferenceZero() != value {
				res.SetIsDifferenceZero(value)
			}
			return res
		})

	h.AccountBankStatement().Methods().ComputeCurrency().DeclareMethod(
		`ComputeCurrency`,
		func(rs m.AccountBankStatementSet) m.AccountBankStatementData {
			res := h.AccountBankStatement().NewData()
			value := rs.Journal().Currency()
			if value.IsEmpty() {
				value = rs.Company().Currency()
			}
			if !rs.Currency().Equals(value) {
				res.SetCurrency(value)
			}
			return res
		})

	h.AccountBankStatement().Methods().CheckLinesReconciled().DeclareMethod(
		`CheckLinesReconciled`,
		func(rs m.AccountBankStatementSet) m.AccountBankStatementData {
			value := true
			for _, line := range rs.Lines().Records() {
				if line.JournalEntries().IsEmpty() && line.Account().IsEmpty() {
					value = false
					break
				}
			}
			res := h.AccountBankStatement().NewData()
			if rs.AllLinesReconciled() != value {
				res.SetAllLinesReconciled(value)
			}
			return res
		})

	h.AccountBankStatement().Methods().DefaultJournal().DeclareMethod(
		`DefaultJournal`,
		func(rs m.AccountBankStatementSet) m.AccountJournalSet {
			journalType := rs.Env().Context().GetString("journal_type")
			company := h.Company().NewSet(rs.Env()).CompanyDefaultGet()
			if journalType != "" {
				query := q.AccountJournal().Type().Equals(journalType).And().Company().Equals(company)
				journals := h.AccountJournal().Search(rs.Env(), query)
				if !journals.IsEmpty() {
					return journals.Records()[0]
				}
			}
			return h.AccountJournal().NewSet(rs.Env())
		})

	h.AccountBankStatement().Methods().GetOpeningBalance().DeclareMethod(
		`GetOpeningBalance`,
		func(rs m.AccountBankStatementSet, journal m.AccountJournalSet) float64 {
			lastBnkStmt := rs.Search(q.AccountBankStatement().Journal().Equals(journal)).Limit(1)
			if !lastBnkStmt.IsEmpty() {
				return lastBnkStmt.BalanceEnd()
			}
			return 0
		})

	h.AccountBankStatement().Methods().DefineOpeningBalance().DeclareMethod(
		`DefineOpeningBalance`,
		func(rs m.AccountBankStatementSet, journal m.AccountJournalSet) {
			val := rs.GetOpeningBalance(journal)
			if rs.BalanceStart() != val {
				rs.SetBalanceStart(val)
			}
		})

	h.AccountBankStatement().Methods().OnchangeJournal().DeclareMethod(
		`OnchangeJournalId`,
		func(rs m.AccountBankStatementSet) m.AccountBankStatementData {
			res := h.AccountBankStatement().NewData()
			rs.DefineOpeningBalance(rs.Journal())
			return res
		})

	h.AccountBankStatement().Methods().BalanceCheck().DeclareMethod(
		`BalanceCheck`,
		func(rs m.AccountBankStatementSet) bool {
			for _, stmt := range rs.Records() {
				if !stmt.Currency().IsZero(stmt.Difference()) {
					if stmt.JournalType() == "cash" {
						var account m.AccountAccountSet
						var name string
						if stmt.Difference() < 0.0 {
							account = stmt.Journal().LossAccount()
							name = rs.T("Loss")
						} else {
							account = stmt.Journal().ProfitAccount()
							name = rs.T("Profit")
						}
						if account.IsEmpty() {
							panic(rs.T(`There is no account defined on the journal %s for %s involved in a cash difference.`, stmt.Journal().Name(), name))
						}
						values := h.AccountBankStatementLine().NewData()
						values.SetStatement(stmt)
						values.SetAccount(account)
						values.SetAmount(stmt.Difference())
						values.SetName(rs.T(`Cash difference observed during the counting (%s)`, name))
						h.AccountBankStatementLine().NewSet(rs.Env()).Create(values)
					} else {
						stmt.Currency()
						blcEndReal := FormatLang(rs.Env(), stmt.BalanceEndReal(), stmt.Currency())
						blcEnd := FormatLang(rs.Env(), stmt.BalanceEnd(), stmt.Currency())
						panic(rs.T(`The ending balance is incorrect !\nThe expected balance (%s) is different from the computed one. (%s)`, blcEndReal, blcEnd))
					}
				}
			}
			return true
		})

	h.AccountBankStatement().Methods().Unlink().Extend("",
		func(rs m.AccountBankStatementSet) int64 {
			for _, stmt := range rs.Records() {
				if stmt.State() != "open" {
					panic(rs.T(`In order to delete a bank statement, you must first cancel it to delete related journal items.`))
				}
				// Explicitly unlink bank statement lines so it will check that the related journal entries have been deleted first
				stmt.Lines().Unlink()
			}
			return rs.Super().Unlink()
		})

	h.AccountBankStatement().Methods().OpenCashboxId().DeclareMethod(
		`OpenCashboxId`,
		func(rs m.AccountBankStatementSet) *actions.Action {
			cashBoxId := rs.Env().Context().GetInteger("cashbox_id")
			if cashBoxId > 0 {
				rs.Env().Context().WithKey("active_id", rs.ID())
				return &actions.Action{
					Name:     rs.T("Cash Control"),
					ViewMode: "form",
					Model:    "AccountBankStatementCashbox",
					View:     views.MakeViewRef("account_view_account_bnk_stmt_cashbox"),
					Type:     actions.ActionActWindow,
					ResID:    cashBoxId,
					Context:  rs.Env().Context(),
					Target:   "new",
				}
			}
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

	h.AccountBankStatement().Methods().ButtonCancel().DeclareMethod(
		`ButtonCancel`,
		func(rs m.AccountBankStatementSet) {
			for _, stmt := range rs.Records() {
				for _, line := range stmt.Lines().Records() {
					if !line.JournalEntries().IsEmpty() {
						panic(rs.T(`A statement cannot be canceled when its lines are reconciled.`))
					}
				}
			}
			rs.SetState("open")
		})

	h.AccountBankStatement().Methods().CheckConfirmBank().DeclareMethod(
		`CheckConfirmBank`,
		func(rs m.AccountBankStatementSet) {
			//@api.multi
			/*def check_confirm_bank(self):
			  if self.journal_type == 'cash' and not self.currency_id.is_zero(self.difference):
			      action_rec = self.env['ir.model.data'].xmlid_to_object('account.action_view_account_bnk_stmt_check') //tovalid this line
			      if action_rec:
			          action = action_rec.read([])[0]
			          return action					tovalid two different return type?
			  return self.button_confirm_bank()
			*/
		})

	h.AccountBankStatement().Methods().ButtonConfirmBank().DeclareMethod(
		`ButtonConfirmBank`,
		func(rs m.AccountBankStatementSet) {
			rs.BalanceCheck()
			statements := rs.Filtered(func(rs m.AccountBankStatementSet) bool { return rs.State() == "open" })
			for _, stmt := range statements.Records() {
				moves := h.AccountMove().NewSet(rs.Env())
				for _, stLine := range stmt.Lines().Records() {
					if !stLine.Account().IsEmpty() && !stLine.JournalEntries().IsEmpty() {
						stLine.FastCounterpartCreation()
					} else if stLine.JournalEntries().IsEmpty() {
						panic(rs.T(`All the account entries lines must be processed in order to close the statement.`))
					}
					moves.Union(stLine.JournalEntries())
				}
				if !moves.IsEmpty() {
					moves.Post()
				}
				//tovalid statement.message_post(body=_('Statement %s confirmed, journal items were created.') % (statement.name,))
			}
			statements.LinkBankToPartner()
			data := h.AccountBankStatement().NewData()
			data.SetState("confirm")
			data.SetDateDone(dates.Now())
			statements.Write(data)

		})

	h.AccountBankStatement().Methods().ButtonJournalEntries().DeclareMethod(
		`ButtonJournalEntries`,
		func(rs m.AccountBankStatementSet) *actions.Action {
			ctx := rs.Env().Context().WithKey("journal_id", rs.Journal().ID())
			return &actions.Action{
				Type:     actions.ActionActWindow,
				Name:     rs.T("Journal Entries"),
				ViewMode: "tree,form",
				Model:    "AccountMove",
				Domain:   "[('id', 'in', self.mapped('move_line_ids').mapped('move_id').ids)]",
				Context:  ctx,
			}
		})

	h.AccountBankStatement().Methods().ButtonOpen().DeclareMethod(
		`Changes statement state to Running.`,
		func(rs m.AccountBankStatementSet) {
			for _, stmt := range rs.Records() {
				data := h.AccountBankStatement().NewData()
				if stmt.Name() == "" {
					ctx := types.NewContext().WithKey("ir_sequence_date", stmt.Date())
					if !stmt.Journal().EntrySequence().IsEmpty() {
						data.SetName(stmt.Journal().EntrySequence().WithNewContext(ctx).NextByID())
					} else {
						seqObj := h.Sequence().NewSet(rs.Env())
						data.SetName(seqObj.WithNewContext(ctx).NextByCode("account.bank.statement")) //tovalid arg snake case?
					}
				}
				data.SetState("open")
				stmt.Write(data)
			}
		})

	h.AccountBankStatement().Methods().ReconciliationWidgetPreprocess().DeclareMethod(
		`Get statement lines of the specified statements or all unreconciled statement lines and try to automatically reconcile them / find them a partner.
			      Return ids of statement lines left to reconcile and other data for the reconciliation widget.`,
		func(rs m.AccountBankStatementSet) (m.AccountBankStatementLineSet, []string, string, int) {
			//NB : The field account_id can be used at the statement line creation/import to avoid the reconciliation process on it later on,
			//this is why we filter out statements lines where account_id is set
			query := q.AccountBankStatementLine().Account().IsNull().
				And().JournalEntries().IsNull().
				And().Company().Equals(h.User().NewSet(rs.Env()).Company())
			if !rs.IsEmpty() {
				query = query.And().Statement().In(rs)
			}
			stLinesLeft := h.AccountBankStatementLine().Search(rs.Env(), query)
			//try to assign partner to bank_statement_line
			stlToAssignPartner := h.AccountBankStatementLine().NewSet(rs.Env())
			var refs []string
			for _, stl := range stLinesLeft.Records() {
				if !stl.Partner().IsEmpty() {
					stlToAssignPartner = stlToAssignPartner.Union(stl)
					if stl.Name() != "" {
						refs = append(refs, stl.Name())
					}

				}
			}
			if journal := stLinesLeft.Journal(); len(refs) > 0 && !journal.DefaultCreditAccount().IsEmpty() && !journal.DefaultDebitAccount().IsEmpty() {
				sqlQuery := `SELECT aml.partner_id, stl.id
			                      FROM account_move_line aml
			                          JOIN account_account acc ON acc.id = aml.account_id
			                          JOIN account_bank_statement_line stl ON aml.ref = stl.name
                                      LEFT JOIN account_account_type aat ON acc.user_type_id = aat.id
			                      WHERE (acc.company_id = %s
			                          AND aml.partner_id IS NOT NULL)
			                          AND (
			                              (aml.statement_id IS NULL AND aml.account_id IN %s)
			                              OR
			                              (aat.type IN ('payable', 'receivable') AND aml.reconciled = false)
			                              )
			                          AND aml.ref IN %s
`
				args := []interface{}{
					h.User().NewSet(rs.Env()).CurrentUser().Company().ID(),
					[]int64{journal.DefaultDebitAccount().ID(), journal.DefaultCreditAccount().ID()},
					refs}
				if !rs.IsEmpty() {
					sqlQuery += "AND stl.id IN %s"
					args = append(args, stlToAssignPartner.Ids())
				}
				var results []struct {
					partner_id int64
					id         int64
				}
				rs.Env().Cr().Select(&results, sqlQuery, args...)
				stL := h.AccountBankStatementLine()
				for _, res := range results {
					data := stL.NewData()
					data.SetPartner(h.Partner().Browse(rs.Env(), []int64{res.partner_id}))
					stL.Browse(rs.Env(), []int64{res.id}).Write(data)
				}
			}
			return stLinesLeft, []string{}, rs.Name(), 0
		})

	h.AccountBankStatement().Methods().LinkBankToPartner().DeclareMethod(
		`LinkBankToPartner`,
		func(rs m.AccountBankStatementSet) {
			for _, stmt := range rs.Records() {
				for _, stL := range stmt.Lines().Records() {
					if !stL.BankAccount().IsEmpty() && !stL.Partner().IsEmpty() && !stL.BankAccount().Partner().Equals(stL.Partner()) {
						stL.BankAccount().SetPartner(stL.Partner())
					}
				}
			}
		})

	h.AccountBankStatementLine().DeclareModel()
	h.AccountBankStatementLine().SetDefaultOrder("Statement DESC", "Sequence", "ID DESC")
	//_inherit = ['ir.needaction_mixin']

	h.AccountBankStatementLine().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:   "Label",
			Required: true},
		"Date": models.DateField{
			Required: true,
			Default: func(env models.Environment) interface{} {
				date := dates.Today()
				if env.Context().HasKey("date") {
					date = env.Context().GetDate("date")
				}
				return date
			}},
		"Amount": models.FloatField{
			Constraint: h.AccountBankStatementLine().Methods().CheckAmount()},
		"JournalCurrency": models.Many2OneField{
			RelationModel: h.Currency(),
			Related:       "Statement.Currency",
			Help:          "Utility field to express amount currency",
			ReadOnly:      true},
		"Partner": models.Many2OneField{
			RelationModel: h.Partner()},
		"BankAccount": models.Many2OneField{
			RelationModel: h.BankAccount()},
		"Account": models.Many2OneField{
			String:        "Counterpart Account",
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Help: `This technical field can be used at the statement line creation/import time in order
to avoid the reconciliation process on it later on. The statement line will simply
create a counterpart on this account`},
		"Statement": models.Many2OneField{
			RelationModel: h.AccountBankStatement(),
			Index:         true,
			Required:      true,
			OnDelete:      models.Cascade},
		"Journal": models.Many2OneField{
			RelationModel: h.AccountJournal(),
			Related:       "Statement.Journal",
			ReadOnly:      true},
		"PartnerName": models.CharField{
			Help: `This field is used to record the third party name when importing bank statement in electronic format,
when the partner doesn't exist yet in the database (or cannot be found).`},
		"Ref": models.CharField{
			String: "Reference"},
		"Note": models.TextField{
			String: "Notes"},
		"Sequence": models.IntegerField{
			Index:   true,
			Help:    "Gives the sequence order when displaying a list of bank statement lines.",
			Default: models.DefaultValue(1)},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Related:       "Statement.Company",
			ReadOnly:      true},
		"JournalEntries": models.One2ManyField{
			RelationModel: h.AccountMove(),
			ReverseFK:     "StatementLine",
			JSON:          "journal_entry_ids",
			ReadOnly:      true},
		"AmountCurrency": models.FloatField{
			Constraint: h.AccountBankStatementLine().Methods().CheckAmount(),
			Help:       "The amount expressed in an optional other currency if it is a multi-currency entry."},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency(),
			Help:          "The optional other currency if it is a multi-currency entry."},
		"State": models.SelectionField{
			Related:  "Statement.State",
			String:   "Status",
			ReadOnly: true},
		"MoveName": models.CharField{
			String:   "Journal Entry Name",
			ReadOnly: true,
			Default:  models.DefaultValue(false),
			NoCopy:   true,
			Help: `Technical field holding the number given to the journal entry, automatically set when the statement line
is reconciled then stored to set the same number again if the line is cancelled,
set to draft and re-processed again.`},
	})

	h.AccountBankStatementLine().Methods().CheckAmount().DeclareMethod(
		`CheckAmount`,
		func(rs m.AccountBankStatementLineSet) {
			//This constraint could possibly underline flaws in bank statement import (eg. inability to
			//support hacks such as using dummy transactions to give additional informations)
			if rs.IsEmpty() {
				panic(rs.T(`A transaction can\'t have a 0 amount.`))
			}
		})

	h.AccountBankStatementLine().Methods().Create().Extend("",
		func(rs m.AccountBankStatementLineSet, data m.AccountBankStatementLineData) m.AccountBankStatementLineSet {
			line := rs.Super().Create(data)
			// The most awesome copy-pasta you will ever see is below. //tovalid
			// Explanation (lel): during a 'create', the 'convert_to_cache' method is not called. Moreover, at
			// that point 'journal_currency_id' is not yet known since it is a related field. It means
			// that the 'amount' field will not be properly rounded. The line below triggers a write on
			// the 'amount' field, which will trigger the 'convert_to_cache' method, and ultimately round
			// the field correctly.
			// This is obviously an awful workaround, but at the time of writing, the ORM does not
			// provide a clean mechanism to fix the issue.
			//line.SetAmount(line.Amount())
			return line
		})

	h.AccountBankStatementLine().Methods().Unlink().Extend("",
		func(rs m.AccountBankStatementLineSet) int64 {
			for _, line := range rs.Records() {
				if !line.JournalEntries().IsEmpty() {
					panic(rs.T(`In order to delete a bank statement line, you must first cancel it to delete related journal items.`))
				}
			}
			return rs.Super().Unlink()
		})

	//h.AccountBankStatementLine().Methods().NeedactionDomainGet().DeclareMethod(
	//	`NeedactionDomainGet`,
	//	func(rs m.AccountBankStatementLineSet) {
	//		//@api.model
	//		/*def _needaction_domain_get(self):
	//		  return [('journal_entry_ids', '=', False), ('account_id', '=', False)]
	//
	//		*/
	//	})

	h.AccountBankStatementLine().Methods().ButtonCancelReconciliation().DeclareMethod(
		`ButtonCancelReconciliation`,
		func(rs m.AccountBankStatementLineSet) {
			movesToCancel := h.AccountMove().NewSet(rs.Env())
			payToUnreconcile := h.AccountPayment().NewSet(rs.Env())
			payToCancel := h.AccountPayment().NewSet(rs.Env())
			for _, stL := range rs.Records() {
				movesToUnbind := stL.JournalEntries()
				for _, move := range movesToUnbind.Records() {
					for _, line := range move.Lines().Records() {
						payToUnreconcile = payToUnreconcile.Union(line.Payment())
						if line.Payment().PaymentReference() == stL.MoveName() {
							//there can be several moves linked to a statement line but maximum one created by the line itself
							movesToCancel = movesToCancel.Union(move)
							payToCancel = payToCancel.Union(line.Payment())
						}
					}
				}
				movesToUnbind = movesToUnbind.Subtract(movesToCancel)
				data := h.AccountMove().NewData()
				data.SetStatementLine(h.AccountBankStatementLine().NewSet(rs.Env()))
				movesToUnbind.Write(data)
				for _, move := range movesToUnbind.Records() {
					moveLine := move.Lines().Filtered(func(rs m.AccountMoveLineSet) bool { return rs.Statement().Equals(stL.Statement()) })
					dataLine := h.AccountMoveLine().NewData()
					dataLine.SetStatement(h.AccountBankStatement().NewSet(rs.Env()))
					moveLine.Write(dataLine)
				}
			}
			payToUnreconcile = payToUnreconcile.Subtract(payToCancel)
			payToUnreconcile.Unreconcile()
			payToCancel.Unlink()
			for _, move := range movesToCancel.Records() {
				move.Lines().RemoveMoveReconcile()
			}
			movesToCancel.ButtonCancel()
			movesToCancel.Unlink()

		})

	h.AccountBankStatementLine().Methods().ReconciliationWidgetAutoReconcile().DeclareMethod(
		`ReconciliationWidgetAutoReconcile`,
		func(rs m.AccountBankStatementLineSet, numAlreadyReconciledLines int) (m.AccountBankStatementLineSet, []map[string]interface{}, string, int) {
			var out struct {
				StL                    m.AccountBankStatementLineSet
				Notifs                 []map[string]interface{}
				NumAlrReconciliedLines int
			}
			autoRecEntries := h.AccountBankStatementLine().NewSet(rs.Env())
			out.StL = h.AccountBankStatementLine().NewSet(rs.Env())
			for _, stl := range rs.Records() {
				if !stl.AutoReconcile().IsEmpty() {
					autoRecEntries.Union(stl)
				} else {
					out.StL.Union(stl)
				}
			}
			//Collect various informations for the reconciliation widget
			autoRecEntriesLen := autoRecEntries.Len()
			if autoRecEntriesLen > 0 {
				msg := rs.T(`1 transaction was automatically reconciled.`)
				if autoRecEntriesLen > 1 {
					msg = rs.T(`%d transactions were automatically reconciled.`, autoRecEntriesLen)
				}
				out.Notifs = append(out.Notifs, map[string]interface{}{
					"Type":    "info",
					"Message": msg,
					"Details": map[string]interface{}{
						"Name":  rs.T(`Automatically reconciled items`),
						"Model": "AccountMove",
						"Rs":    autoRecEntries.JournalEntries()}})
			}
			return out.StL, out.Notifs, "", out.NumAlrReconciliedLines
		})

	h.AccountBankStatementLine().Methods().GetDataForReconciliationWidget().DeclareMethod(
		`Returns the data required to display a reconciliation widget, for each statement line in self`,
		func(rs m.AccountBankStatementLineSet, excludedIds []int64) (m.AccountBankStatementLineData, m.AccountMoveLineData) {
			for _, stl := range rs.Records() {
				amlRecs := stl.GetReconciliationProposition(excludedIds)
				var tgtCurrency m.CurrencySet
				tgtCurrency = h.Currency().Coalesce(stl.Currency(), stl.Journal().Currency(), stl.Journal().Company().Currency())
				rp := amlRecs.PrepareMoveLinesForReconciliationWidget(tgtCurrency, stl.Date())
				for _, ml := range rp {
					excludedIds = append(excludedIds, ml["id"].(int64))
				}
				/*
					ret.append({
							'st_line': st_line.get_statement_line_for_reconciliation_widget(),
							'reconciliation_proposition': rp
						})
				*/
			}
			// FIXME
			return h.AccountBankStatementLine().NewData(), h.AccountMoveLine().NewData() // tovalid return values
		})

	h.AccountBankStatementLine().Methods().GetStatementLineForReconciliationWidget().DeclareMethod(
		`Returns the data required by the bank statement reconciliation widget to display a statement line`,
		func(rs m.AccountBankStatementLineSet) map[string]interface{} {
			stmtCurrency := h.Currency().Coalesce(rs.Journal().Currency(), rs.Journal().Company().Currency())
			amtCurrencyStr := ""
			amount := rs.Amount()
			if rs.AmountCurrency() != 0.0 && rs.Currency().IsNotEmpty() {
				amount = rs.AmountCurrency()
				amountCurrency := math.Abs(rs.Amount())
				amtCurrencyStr = FormatLang(rs.Env(), amountCurrency, stmtCurrency)
			}
			amountStr := FormatLang(rs.Env(), math.Abs(amount), h.Currency().Coalesce(rs.Currency(), stmtCurrency))
			data := map[string]interface{}{
				"id":                         rs.ID(),
				"ref":                        rs.Ref(),
				"note":                       rs.Note(),
				"name":                       rs.Name,
				"date":                       rs.Date(),
				"amount":                     amount,
				"amount_str":                 amountStr, // Amount in the statement line currency
				"currency_id":                h.Currency().Coalesce(rs.Currency(), stmtCurrency).ID(),
				"partner_id":                 rs.Partner().ID(),
				"journal_id":                 rs.Journal().ID(),
				"statement_id":               rs.Statement().ID(),
				"account_code":               rs.Journal().DefaultDebitAccount().Code(),
				"account_name":               rs.Journal().DefaultDebitAccount().Name(),
				"partner_name":               rs.Partner().Name(),
				"communication_partner_name": rs.PartnerName(),
				"amount_currency_str":        amtCurrencyStr,
				"has_no_partner":             rs.Partner().IsEmpty(),
			}
			if rs.Partner().IsEmpty() {
				return data
			}
			data["open_balance_account_id"] = rs.Partner().PropertyAccountPayable().ID()
			if amount > 0 {
				data["open_balance_account_id"] = rs.Partner().PropertyAccountReceivable().ID()
			}
			return data
		})

	h.AccountBankStatementLine().Methods().GetMoveLinesForReconciliationWidget().DeclareMethod(
		`Returns move lines for the bank statement reconciliation widget, formatted as a list of dicts`,
		func(rs m.AccountBankStatementLineSet, excludedIds []int64, str string, offset int, limit int) []map[string]interface{} {
			amlRecs := rs.GetMoveLinesForReconciliation(excludedIds, str, offset, limit, q.AccountMoveLineCondition{}, h.Partner().NewSet(rs.Env()))
			tgtCurrency := h.Currency().Coalesce(rs.Currency(), rs.Journal().Currency(), rs.Journal().Company().Currency())
			return amlRecs.PrepareMoveLinesForReconciliationWidget(tgtCurrency, rs.Date())
		})

	h.AccountBankStatementLine().Methods().GetMoveLinesForReconciliation().DeclareMethod(
		`Return account.move.line records which can be used for bank statement reconciliation.

			      :param excluded_ids:
			      :param str:
			      :param offset:
			      :param limit:
			      :param additional_domain:
			      :param overlook_partner:`,
		func(rs m.AccountBankStatementLineSet, excludedIds []int64, str string, offset, limit int,
			additionalCond q.AccountMoveLineCondition, overlookPartner m.PartnerSet) m.AccountMoveLineSet {
			// Blue lines = payment on bank account not assigned to a statement yet
			qDomainReconciliation := q.AccountMoveLine().Statement().IsNull().
				And().Payment().IsNotNull().
				AndCond(q.AccountMoveLine().Account().In(rs.Journal().DefaultCreditAccount()).
					Or().Account().In(rs.Journal().DefaultDebitAccount()))
			qDomainMatching := q.AccountMoveLine().Reconciled().Equals(false)
			if rs.Partner().IsNotEmpty() || overlookPartner.IsNotEmpty() {
				//qDomainMatching = qDomainMatching.And().Account() //tovalid expression.AND([domain_matching, [('account_id.internal_type', 'in', ['payable', 'receivable'])]])
			} else {
				//TODO : find out what use case this permits (match a check payment, registered on a journal whose account type is other instead of liquidity)
				//qDomainMatching = qDomainMatching.And().Account() //tovalid expression.AND([domain_matching, [('account_id.reconcile', '=', True)]])
			}
			// Let's add what applies to both
			query := qDomainReconciliation.OrCond(qDomainMatching)
			if rs.Partner().IsNotEmpty() && overlookPartner.IsEmpty() {
				query = query.And().Partner().Equals(rs.Partner())
			}
			// Domain factorized for all reconciliation use cases
			gnrcDomain := h.AccountMoveLine().NewSet(rs.Env()).WithContext("bank_statement_line_id", rs.ID()).DomainMoveLinesForReconciliation(excludedIds, str)
			query = query.AndCond(gnrcDomain)
			// Domain from caller
			query = query.AndCond(additionalCond)
			return h.AccountMoveLine().Search(rs.Env(), query).Offset(offset).Limit(limit).OrderBy("date_maturity", "asc,", "id", "asc") //tovalid order="date_maturity asc, id asc"
		})

	h.AccountBankStatementLine().Methods().GetCommonSqlQuery().DeclareMethod(
		`GetCommonSqlQuery`,
		func(rs m.AccountBankStatementLineSet, overlookPartner bool, excludedIds []int64) (string, string, string) {
			accType := `acc.reconcile = true`
			if rs.Partner().IsNotEmpty() || overlookPartner {
				accType = `aat.type IN ('payable', 'receivable')`
			}
			accClause := ``
			if rs.Journal().DefaultDebitAccount().IsNotEmpty() && rs.Journal().DefaultCreditAccount().IsNotEmpty() {
				accClause = `(aml.statement_id IS NULL AND aml.account_id IN (:account_payable_receivable) AND aml.payment_id IS NOT NULL) OR`
			}
			whereClause := fmt.Sprintf(`WHERE acc.company_id = :company_id
				AND ( %s (%s AND aml.reconciled = false))`, accClause, accType)
			if rs.Partner().IsNotEmpty() {
				whereClause += ` AND aml.partner_id = :partner_id`
			}
			if len(excludedIds) > 0 {
				whereClause += ` AND aml.id NOT IN (:excluded_ids)`
			}
			return `SELECT aml.id `,
				`FROM account_move_line aml 
				JOIN account_account acc ON acc.id = aml.account_id
				LEFT JOIN account_account_type aat ON acc.user_type_id = aat.id `,
				whereClause
		})

	h.AccountBankStatementLine().Methods().GetReconciliationProposition().DeclareMethod(
		`Returns move lines that constitute the best guess to reconcile a statement line
					Note: it only looks for move lines in the same currency as the statement line.`,
		func(rs m.AccountBankStatementLineSet, excludedIds []int64) m.AccountMoveLineSet {
			rs.EnsureOne()
			amount := rs.AmountCurrency()
			if amount == 0 {
				amount = rs.Amount()
			}
			companyCurrency := rs.Journal().Company().Currency()
			stLineCurrency := h.Currency().Coalesce(rs.Currency(), rs.Journal().Currency())
			currency := h.Currency().NewSet(rs.Env())
			if stLineCurrency.IsNotEmpty() && !stLineCurrency.Equals(companyCurrency) {
				currency = stLineCurrency
			}
			precision := companyCurrency.DecimalPlaces()
			if stLineCurrency.IsNotEmpty() {
				precision = stLineCurrency.DecimalPlaces()
			}
			_, _ = currency, precision
			params := map[string]interface{}{
				"company_id":                 h.User().NewSet(rs.Env()).CurrentUser().Company().ID(),
				"account_payable_receivable": []int64{rs.Journal().DefaultCreditAccount().ID(), rs.Journal().DefaultDebitAccount().ID()},
				"amount":                     fmt.Sprintf(fmt.Sprintf("%%.%df", precision), amount),
				"partner_id":                 rs.Partner().ID(),
				"excluded_ids":               excludedIds,
				"ref":                        rs.Name(),
			}
			type resStruct struct {
				ID             int64 `db:"id"`
				TempFieldOrder int   `db:"temp_field_order"`
			}
			// Look for structured communication match
			if rs.Name() != "" {
				addToSelect := ", CASE WHEN aml.ref = :ref THEN 1 ELSE 2 END as temp_field_order "
				addToFrom := " JOIN account_move m ON m.id = aml.move_id "
				selectClause, fromClause, whereClause := rs.GetCommonSqlQuery(true, excludedIds)
				query := selectClause + addToSelect + fromClause + addToFrom + whereClause
				query += " AND (aml.ref= :ref or m.name = :ref) ORDER BY temp_field_order, date_maturity asc, aml.id asc"
				slQuery, slParams, err := sqlx.Named(query, params)
				if err != nil {
					panic(rs.T("Unable to bind query with named parameters.\nError: %s\nQuery: %s \nParams: %v", err, query, params))
				}
				var res []resStruct
				rs.Env().Cr().Select(&res, slQuery, slParams...)
				if len(res) > 0 {
					return h.AccountMoveLine().BrowseOne(rs.Env(), res[0].ID)
				}
			}

			// Look for a single move line with the same amount
			field := h.AccountMoveLine().Fields().AmountResidualCurrency()
			if currency.IsEmpty() {
				field = h.AccountMoveLine().Fields().AmountResidual()
			}
			liquidityField := h.AccountMoveLine().Fields().AmountCurrency()
			if currency.IsEmpty() {
				liquidityField = h.AccountMoveLine().Fields().Credit()
				if amount > 0 {
					liquidityField = h.AccountMoveLine().Fields().Debit()
				}
			}
			liquidityAmtClause := "CAST(:amount AS numeric)"
			if currency.IsEmpty() {
				liquidityAmtClause = "CAST(abs(:amount) AS numeric)"
			}
			selectClause, fromClause, whereClause := rs.GetCommonSqlQuery(true, excludedIds)
			query := selectClause + fromClause + whereClause
			query += fmt.Sprintf(" AND (aml.%s = CAST(:amount AS numeric) OR (aat.type = 'liquidity' AND aml.%s = %s )) ORDER BY date_maturity asc, aml.id asc LIMIT 1",
				h.AccountMoveLine().Model.JSONizeFieldName(field.String()),
				h.AccountMoveLine().Model.JSONizeFieldName(liquidityField.String()),
				liquidityAmtClause)
			slQuery, slParams, err := sqlx.Named(query, params)
			if err != nil {
				panic(rs.T("Unable to bind query with named parameters.\nError: %s\nQuery: %s \nParams: %v", err, query, params))
			}
			var res []resStruct
			rs.Env().Cr().Select(&res, slQuery, slParams...)
			return h.AccountMoveLine().BrowseOne(rs.Env(), res[0].ID)
		})

	h.AccountBankStatementLine().Methods().GetMoveLinesForAutoReconcile().DeclareMethod(
		`Returns the move lines that the method auto_reconcile can use to try to reconcile the statement line`,
		func(rs m.AccountBankStatementLineSet) m.AccountMoveLineSet {
			return h.AccountMoveLine().NewSet(rs.Env())
		})

	h.AccountBankStatementLine().Methods().AutoReconcile().DeclareMethod(
		`Try to automatically reconcile the statement.line ; return the counterpart journal entry/ies if the automatic reconciliation succeeded, False otherwise.
            TODO : this method could be greatly improved and made extensible`,
		func(rs m.AccountBankStatementLineSet) m.AccountMoveLineSet {
			rs.EnsureOne()

			amount := rs.AmountCurrency()
			if amount == 0.0 {
				amount = rs.Amount()
			}

			companyCurrency := rs.Journal().Company().Currency()

			stLineCurrency := rs.Currency()
			if stLineCurrency.IsEmpty() {
				stLineCurrency = rs.Journal().Currency()
			}

			currency := h.Currency().NewSet(rs.Env())
			if stLineCurrency.IsNotEmpty() && !stLineCurrency.Equals(companyCurrency) {
				currency = stLineCurrency
			}

			precision := companyCurrency.DecimalPlaces()
			if stLineCurrency.IsNotEmpty() {
				precision = stLineCurrency.DecimalPlaces()
			}

			params := map[string]interface{}{
				"company_id":                 h.User().NewSet(rs.Env()).CurrentUser().Company().ID(),
				"account_payable_receivable": []int64{rs.Journal().DefaultDebitAccount().ID(), rs.Journal().DefaultCreditAccount().ID()},
				"amount":                     nbutils.Round(amount, nbutils.Digits{Precision: int8(precision), Scale: 0}.ToPrecision()),
				"partner_id":                 rs.Partner().ID(),
				"ref":                        rs.Name(),
			}

			field := "amount_residual"
			liquidityField := "credit"
			if amount > 0.0 {
				liquidityField = "debit"
			}
			if currency.IsNotEmpty() {
				field = "amount_residual_currency"
				liquidityField = "amount_currency"
			}

			var queryOut []struct {
				Id int64
			}

			// Look for structured communication match
			if rs.Name() != "" {
				/*
					sql_query = self._get_common_sql_query() + \  #tovalid same as above: named parameters
						" AND aml.ref = %(ref)s AND ("+field+" = %(amount)s OR (acc.internal_type = 'liquidity' AND "+liquidity_field+" = %(amount)s)) \
						ORDER BY date_maturity asc, aml.id asc"
					self.env.cr.execute(sql_query, params)
					query_out = self.env.cr.dictfetchall()
					if len(query_out) > 1:
						return False
				*/
			}

			// Look for a single move line with the same partner, the same amount
			if len(queryOut) == 0 && rs.Partner().IsNotEmpty() {
				/*
					sql_query = self._get_common_sql_query() + \
					" AND ("+field+" = %(amount)s OR (acc.internal_type = 'liquidity' AND "+liquidity_field+" = %(amount)s)) \
					ORDER BY date_maturity asc, aml.id asc"
					self.env.cr.execute(sql_query, params)
					query_out = self.env.cr.dictfetchall()
					if len(query_out) > 1:
						return False
				*/
			}

			if len(queryOut) == 0 {
				return h.AccountMoveLine().NewSet(rs.Env())
			}

			var ids []int64
			for _, aml := range queryOut {
				ids = append(ids, aml.Id)
			}
			matchRecs := h.AccountMoveLine().Browse(rs.Env(), ids)

			// Now reconcile

			/*
				counterpart_aml_dicts = []
				payment_aml_rec = self.env['account.move.line']
				for aml in match_recs:
					if aml.account_id.internal_type == 'liquidity':
						payment_aml_rec = (payment_aml_rec | aml)
					else:
						amount = aml.currency_id and aml.amount_residual_currency or aml.amount_residual
						counterpart_aml_dicts.append({
							'name': aml.name if aml.name != '/' else aml.move_id.name,
							'debit': amount < 0 and -amount or 0,
							'credit': amount > 0 and amount or 0,
							'move_line': aml
						})

				try:
					with self._cr.savepoint():
						counterpart = self.process_reconciliation(counterpart_aml_dicts=counterpart_aml_dicts, payment_aml_rec=payment_aml_rec)
					return counterpart
				except UserError:
					# A configuration / business logic error that makes it impossible to auto-reconcile should not be raised
					# since automatic reconciliation is just an amenity and the user will get the same exception when manually
					# reconciling. Other types of exception are (hopefully) programmation errors and should cause a stacktrace.
					self.invalidate_cache()
					self.env['account.move'].invalidate_cache()
					self.env['account.move.line'].invalidate_cache()
					return False

			*/

			_, _, _, _ = params, field, liquidityField, matchRecs
			return h.AccountMoveLine().NewSet(rs.Env())
		})

	h.AccountBankStatementLine().Methods().PrepareReconciliationMove().DeclareMethod(
		`Prepare the dict of values to create the move from a statement line. This method may be overridden to adapt domain logic
			      through model inheritance (make sure to call super() to establish a clean extension chain).

			     :param char move_ref: will be used as the reference of the generated account move
			     :return: dict of value to create() the account.move`,
		func(rs m.AccountBankStatementLineSet, moveRef string) m.AccountMoveData {
			ref := moveRef
			if rs.Ref() != "" {
				ref = rs.Ref()
				if moveRef != "" {
					ref = moveRef + " - " + rs.Ref()
				}
			}
			data := h.AccountMove().NewData().
				SetStatementLine(rs).
				SetJournal(rs.Statement().Journal()).
				SetDate(rs.Date()).
				SetRef(ref)
			if rs.MoveName() != "" {
				data.SetName(rs.MoveName())
			}
			return data
		})

	h.AccountBankStatementLine().Methods().PrepareReconciliationMoveLine().DeclareMethod(
		`Prepare the dict of values to balance the move.

			      :param recordset move: the account.move to link the move line
			      :param float amount: the amount of transaction that wasn't already reconciled`,
		func(rs m.AccountBankStatementLineSet, move m.AccountMoveSet, amount float64) m.AccountMoveLineData {
			companyCurrency := rs.Journal().Company().Currency()
			statementCurrency := h.Currency().Coalesce(rs.Journal().Currency(), companyCurrency)
			statementLineCurrency := h.Currency().Coalesce(rs.Currency(), statementCurrency)
			var amountCurrency, stLineCurrencyRate float64
			if rs.Currency().IsNotEmpty() {
				stLineCurrencyRate = rs.AmountCurrency() / rs.Amount()
			}
			// We have several use case here to compute the currency and amount currency of counterpart line to balance the move:
			switch {
			case (!statementLineCurrency.Equals(companyCurrency) && statementLineCurrency.Equals(statementCurrency)) || (!statementLineCurrency.Equals(companyCurrency) && statementCurrency.Equals(companyCurrency)):
				// company in currency A, statement in currency B and transaction in currency B or
				// company in currency A, statement in currency A and transaction in currency B
				// counterpart line must have currency B and correct amount is inverse of already existing lines
				for _, line := range move.Lines().Records() {
					amountCurrency -= line.AmountCurrency()
				}
			case !statementLineCurrency.Equals(companyCurrency) && !statementLineCurrency.Equals(statementCurrency):
				// company in currency A, statement in currency B and transaction in currency C
				// counterpart line must have currency B and use rate between B and C to compute correct amount
				for _, line := range move.Lines().Records() {
					amountCurrency -= line.AmountCurrency()
				}
				amountCurrency /= stLineCurrencyRate
			case statementLineCurrency.Equals(companyCurrency) && !statementCurrency.Equals(companyCurrency):
				// company in currency A, statement in currency B and transaction in currency A
				// counterpart line must have currency B and amount is computed using the rate between A and B
				amountCurrency = amount / stLineCurrencyRate
			}

			// last case is company in currency A, statement in currency A and transaction in currency A
			// and in this case counterpart line does not need any second currency nor amount_currency
			data := h.AccountMoveLine().NewData().
				SetName(rs.Name()).
				SetMove(move).
				SetPartner(rs.Partner()).
				SetStatement(rs.Statement()).
				SetAmountCurrency(amountCurrency)
			if amount >= 0 {
				data.SetAccount(rs.Statement().Journal().DefaultCreditAccount())
				data.SetDebit(amount)
			} else {
				data.SetAccount(rs.Statement().Journal().DefaultDebitAccount())
				data.SetCredit(-amount)
			}
			switch {
			case statementCurrency != companyCurrency:
				data.SetCurrency(statementCurrency)
			case statementLineCurrency != companyCurrency:
				data.SetCurrency(statementLineCurrency)
			}
			return data
		})

	h.AccountBankStatementLine().Methods().ProcessReconciliations().DeclareMethod(
		`Handles data sent from the bank statement reconciliation widget (and can otherwise serve as an old-API bridge)

			      :param list of dicts data: must contains the keys 'counterpart_aml_dicts', 'payment_aml_ids' and 'new_aml_dicts',
			          whose value is the same as described in process_reconciliation except that ids are used instead of recordsets.`,
		func(rs m.AccountBankStatementLineSet, data []map[string]interface{}) {
			type Datum struct {
				PaymentAml          m.AccountMoveLineSet
				CounterpartAmlDatas []accounttypes.BankStatementAMLStruct
				NewAmlDatas         []accounttypes.BankStatementAMLStruct
			}
			safifyData := func(data map[string]interface{}) Datum {
				out := Datum{h.AccountMoveLine().NewSet(rs.Env()), []accounttypes.BankStatementAMLStruct{}, []accounttypes.BankStatementAMLStruct{}}
				if valPaymentAmlIds, ok := data["payment_aml_ids"]; ok {
					out.PaymentAml = h.AccountMoveLine().Browse(rs.Env(), valPaymentAmlIds.([]int64))
				}
				if valCounterpartAmlDicts, ok := data["counterpart_aml"]; ok {
					for _, val := range valCounterpartAmlDicts.([]accounttypes.BankStatementAMLStruct) {
						out.CounterpartAmlDatas = append(out.CounterpartAmlDatas, val)
					}
				}
				if valNewAmlDicts, ok := data["new_aml_dicts"]; ok {
					for _, val := range valNewAmlDicts.([]accounttypes.BankStatementAMLStruct) {
						out.NewAmlDatas = append(out.CounterpartAmlDatas, val)
					}
				}
				return out
			}
			stLine := rs.Records()
			for i := 0; i < len(data) && i < rs.Len(); i++ {
				datum := safifyData(data[i])
				for _, amlDict := range datum.CounterpartAmlDatas {
					amlDict.MoveLineID = h.AccountMoveLine().BrowseOne(rs.Env(), amlDict.CounterpartAMLID).ID()
					amlDict.CounterpartAMLID = 0
				}
				stLine[i].ProcessReconciliation(datum.PaymentAml, datum.CounterpartAmlDatas, datum.NewAmlDatas)
			}
		})

	h.AccountBankStatementLine().Methods().FastCounterpartCreation().DeclareMethod(
		`FastCounterpartCreation`,
		func(rs m.AccountBankStatementLineSet) {
			for _, stl := range rs.Records() {
				data := accounttypes.BankStatementAMLStruct{Name: stl.Name(), AccountID: stl.Account().ID()}
				if stl.Amount() < 0 {
					data.Debit = -stl.Amount()
				} else {
					data.Credit = stl.Amount()
				}
				stl.ProcessReconciliation(h.AccountMoveLine().NewSet(rs.Env()), nil, []accounttypes.BankStatementAMLStruct{data})
			}
		})

	h.AccountBankStatementLine().Methods().GetCommunication().DeclareMethod(
		`GetCommunication`,
		func(rs m.AccountBankStatementLineSet, paymentMethod m.AccountPaymentMethodSet) string {
			return rs.Name()
		})

	h.AccountBankStatementLine().Methods().ConvertAMLStructToData().DeclareMethod(
		`ConvertAMLStructToData converts the given BankStatementAMLStruct to m.AccountMoveLineData`,
		func(rs m.AccountBankStatementLineSet, strc accounttypes.BankStatementAMLStruct) m.AccountMoveLineData {
			return h.AccountMoveLine().NewData().
				SetName(strc.Name).
				SetCredit(strc.Credit).
				SetDebit(strc.Debit).
				SetAmountCurrency(strc.AmountCurrency).
				SetAccount(h.AccountAccount().BrowseOne(rs.Env(), strc.AccountID)).
				SetCurrency(h.Currency().BrowseOne(rs.Env(), strc.CurrencyID)).
				SetMove(h.AccountMove().BrowseOne(rs.Env(), strc.MoveID)).
				SetPartner(h.Partner().BrowseOne(rs.Env(), strc.PartnerID)).
				SetStatement(h.AccountBankStatement().BrowseOne(rs.Env(), strc.StatementID)).
				SetPayment(h.AccountPayment().BrowseOne(rs.Env(), strc.PaymentID))
		})

	h.AccountBankStatementLine().Methods().ConvertSetToAmlStruct().DeclareMethod(
		``,
		func(rs m.AccountBankStatementLineSet, set m.AccountMoveLineSet) []accounttypes.BankStatementAMLStruct {
			var out []accounttypes.BankStatementAMLStruct
			for _, s := range set.Records() {
				out = append(out, accounttypes.BankStatementAMLStruct{
					Name:           s.Name(),
					Credit:         s.Credit(),
					Debit:          s.Debit(),
					AmountCurrency: s.AmountCurrency(),
					AccountID:      s.Account().ID(),
					CurrencyID:     s.Currency().ID(),
					MoveID:         s.Move().ID(),
					PartnerID:      s.Partner().ID(),
					StatementID:    s.Statement().ID(),
					PaymentID:      s.Payment().ID(),
				})
			}
			return out
		})

	h.AccountBankStatementLine().Methods().CompleteAMLStructs().DeclareMethod(`
		CompleteAMLStructs takes a slice of BankStatementAMLStruct and returns a new slice with each struct updated.`,
		func(rs m.AccountBankStatementLineSet, AMLStructs []accounttypes.BankStatementAMLStruct, move m.AccountMoveSet) []accounttypes.BankStatementAMLStruct {
			companyCurrency := rs.Journal().Company().Currency()
			statementCurrency := h.Currency().Coalesce(rs.Journal().Currency(), companyCurrency)
			statementLineCurrency := h.Currency().Coalesce(rs.Currency(), statementCurrency)
			var stLineCurrencyRate float64
			if rs.Currency().IsNotEmpty() {
				stLineCurrencyRate = rs.AmountCurrency() / rs.Amount()
			}

			var res []accounttypes.BankStatementAMLStruct
			for _, amlDict := range AMLStructs {
				amlDict.MoveID = move.ID()
				amlDict.PartnerID = rs.Partner().ID()
				amlDict.StatementID = rs.Statement().ID()
				if !statementLineCurrency.Equals(companyCurrency) {
					amlDict.AmountCurrency = amlDict.Debit - amlDict.Credit
					amlDict.CurrencyID = statementLineCurrency.ID()
					switch {
					case rs.Currency().IsNotEmpty() && statementCurrency.Equals(companyCurrency) && stLineCurrencyRate != 0.0:
						// Statement is in company currency but the transaction is in foreign currency
						amlDict.Debit = companyCurrency.Round(amlDict.Debit / stLineCurrencyRate)
						amlDict.Credit = companyCurrency.Round(amlDict.Credit / stLineCurrencyRate)
					case rs.Currency().IsNotEmpty() && stLineCurrencyRate != 0.0:
						// Statement is in foreign currency and the transaction is in another one
						amlDict.Debit = statementLineCurrency.Compute(amlDict.Debit/stLineCurrencyRate, companyCurrency, true)
						amlDict.Credit = statementCurrency.Compute(amlDict.Credit/stLineCurrencyRate, companyCurrency, true)
					default:
						// Statement is in foreign currency and no extra currency is given for the transaction
						amlDict.Debit = statementLineCurrency.Compute(amlDict.Debit, companyCurrency, true)
						amlDict.Credit = statementLineCurrency.Compute(amlDict.Credit, companyCurrency, true)
					}
				} else if !statementCurrency.Equals(companyCurrency) {
					// Statement is in foreign currency but the transaction is in company currency
					prorataFactor := (amlDict.Debit - amlDict.Credit) / rs.AmountCurrency()
					amlDict.AmountCurrency = prorataFactor * rs.Amount()
					amlDict.CurrencyID = statementCurrency.ID()
				}
				res = append(res, amlDict)
			}
			return res
		})

	h.AccountBankStatementLine().Methods().ProcessReconciliation().DeclareMethod(
		`Match statement lines with existing payments (eg. checks) and/or payables/receivables (eg. invoices and refunds) and/or new move lines (eg. write-offs).
			      If any new journal item needs to be created (via new_aml_dicts or counterpart_aml_dicts), a new journal entry will be created and will contain those
			      items, as well as a journal item for the bank statement line.
			      Finally, mark the statement line as reconciled by putting the matched moves ids in the column journal_entry_ids.

			      :param self: browse collection of records that are supposed to have no accounting entries already linked.
				  :param (list of recordsets) payment_aml_rec: recordset move lines representing existing payments (which are already fully reconciled)

			      :param (list of dicts) counterpart_aml_dicts: move lines to create to reconcile with existing payables/receivables.
			          The expected keys are :
			          - 'name'
			          - 'debit'
			          - 'credit'
			          - 'move_line'
			              # The move line to reconcile (partially if specified debit/credit is lower than move line's credit/debit)

			      :param (list of dicts) new_aml_dicts: move lines to create. The expected keys are :
			          - 'name'
			          - 'debit'
			          - 'credit'
			          - 'account_id'
			          - (optional) 'tax_ids'
			          - (optional) Other account.move.line fields like analytic_account_id or analytics_id

			      :returns: The journal entries with which the transaction was matched. If there was at least an entry in counterpart_aml_dicts or new_aml_dicts, this list contains
			          the move created by the reconciliation, containing entries for the statement.line (1), the counterpart move lines (0..*) and the new move lines (0..*).
			  `,
		func(rs m.AccountBankStatementLineSet, paymentAMLRec m.AccountMoveLineSet, counterpartAMLDicts, newAMLDicts []accounttypes.BankStatementAMLStruct) m.AccountMoveSet {
			// Check and prepare recieved data
			if rs.MoveName() != "" {
				panic(rs.T(`Operation not allowed. Since your statement line already received a number, you cannot reconcile it entirely with existing journal entries otherwise it would make a gap in the numbering. You should book an entry and make a regular revert of it in case you want to cancel it.`))
			}
			for _, rc := range paymentAMLRec.Records() {
				if rc.Statement().IsNotEmpty() {
					panic(rs.T(`A selected move line was already reconciled.`))
				}
			}
			for _, amlDict := range counterpartAMLDicts {
				if h.AccountMoveLine().BrowseOne(rs.Env(), amlDict.MoveLineID).Reconciled() {
					panic(rs.T(`A selected move line was already reconciled.`))
				}
			}
			for _, line := range rs.Records() {
				if line.JournalEntries().IsNotEmpty() {
					panic(rs.T(`A selected statement line was already reconciled with an account move.`))
				}
			}

			// Fully reconciled moves are just linked to the bank statement
			total := rs.Amount()
			counterPartMoves := h.AccountMove().NewSet(rs.Env())
			for _, amlRec := range paymentAMLRec.Records() {
				total -= amlRec.Debit() - amlRec.Credit()
				amlRec.SetStatement(rs.Statement())
				amlRec.Move().SetStatementLine(rs)
				counterPartMoves = counterPartMoves.Union(amlRec.Move())
			}
			// Create move line(s). Either matching an existing journal entry (eg. invoice), in which
			// case we reconcile the existing and the new move lines together, or being a write-off.
			if len(counterpartAMLDicts)+len(newAMLDicts) == 0 {
				counterPartMoves.AssertBalanced()
				return counterPartMoves
			}
			companyCurrency := rs.Journal().Company().Currency()

			// Create the move
			ids := rs.Statement().Lines().Ids()
			ID := rs.ID()
			for i, id := range ids {
				if id == ID {
					rs.SetSequence(int64(i + 1))
				}
			}
			moveVals := rs.PrepareReconciliationMove(rs.Statement().Name())
			move := h.AccountMove().Create(rs.Env(), moveVals)
			counterPartMoves = counterPartMoves.Union(move)

			// Create the payment
			payment := h.AccountPayment().NewSet(rs.Env())
			if math.Abs(total) > 0.00001 {
				data := h.AccountPayment().NewData()
				if rs.Partner().IsNotEmpty() {
					data.SetPartner(rs.Partner())
					partnerTypeValue := "customer"
					if total < 0 {
						partnerTypeValue = "supplier"
					}
					data.SetPartnerType(partnerTypeValue)
				}
				paymentMethods := rs.Journal().OutboundPaymentMethods()
				paymentType := "outbound"
				if total > 0 {
					paymentMethods = rs.Journal().InboundPaymentMethods()
					paymentType = "inbound"
				}
				if paymentMethods.IsNotEmpty() {
					data.SetPaymentMethod(paymentMethods.Records()[0])
					data.SetCommunication(rs.GetCommunication(paymentMethods))
				}
				nameVal := rs.T(`Bank Statement %s`, rs.Date())
				if rs.Statement().Name() != "" {
					nameVal = rs.Statement().Name()
				}
				data.
					SetName(nameVal).
					SetPaymentType(paymentType).
					SetCurrency(h.Currency().Coalesce(rs.Journal().Currency(), rs.Company().Currency())).
					SetJournal(rs.Statement().Journal()).
					SetPaymentDate(rs.Date()).
					SetState("reconciled").
					SetAmount(math.Abs(total))
				payment = h.AccountPayment().Create(rs.Env(), data)
			}

			// Complete dicts to create both counterpart move lines and write-offs
			ctx := rs.Env().Context().WithKey("date", rs.Date())
			counterpartAMLDicts = rs.WithNewContext(ctx).CompleteAMLStructs(counterpartAMLDicts, move)
			newAMLDicts = rs.WithNewContext(ctx).CompleteAMLStructs(newAMLDicts, move)

			// Create write-offs
			// When we register a payment on an invoice, the write-off line contains the amount
			// currency if all related invoices have the same currency. We apply the same logic in
			// the manual reconciliation.
			counterpartAML := h.AccountMoveLine().NewSet(rs.Env())
			for _, amlDict := range counterpartAMLDicts {
				counterpartAML = counterpartAML.Union(h.AccountMoveLine().BrowseOne(rs.Env(), amlDict.MoveLineID))
			}
			newAMLCur := h.Currency().NewSet(rs.Env())
			currencies := h.Currency().NewSet(rs.Env())
			for _, aml := range counterpartAML.Records() {
				currencies = currencies.Union(aml.Currency())
			}
			firstCounterpartAML := counterpartAML.First()
			if currencies.Len() == 1 && firstCounterpartAML.HasCurrency() && !firstCounterpartAML.Currency().Equals(companyCurrency) {
				newAMLCur = firstCounterpartAML.Currency()
			}
			for _, amlDict := range newAMLDicts {
				if payment.IsNotEmpty() {
					amlDict.PaymentID = payment.ID()
				}
				if newAMLCur.IsNotEmpty() && h.Currency().BrowseOne(rs.Env(), amlDict.CurrencyID).IsEmpty() {
					amlDict.CurrencyID = newAMLCur.ID()
					amlDict.AmountCurrency = companyCurrency.WithNewContext(ctx).Compute(amlDict.Debit-amlDict.Credit, newAMLCur, true)
				}
				h.AccountMoveLine().NewSet(rs.Env()).
					WithContext("apply_taxes", true).
					WithContext("check_move_validity", false).
					Create(rs.ConvertAMLStructToData(amlDict))
			}

			// Create counterpart move lines and reconcile them
			for _, amlDict := range counterpartAMLDicts {
				counterpartMoveLine := h.AccountMoveLine().BrowseOne(rs.Env(), amlDict.MoveLineID)
				if counterpartMoveLine.Partner().IsNotEmpty() {
					amlDict.PartnerID = counterpartMoveLine.Partner().ID()
				}
				amlDict.AccountID = counterpartMoveLine.Account().ID()
				if payment.IsNotEmpty() {
					amlDict.PaymentID = payment.ID()
				}
				if counterpartMoveLine.Currency().IsNotEmpty() && !counterpartMoveLine.Currency().Equals(companyCurrency) && h.Currency().BrowseOne(rs.Env(), amlDict.CurrencyID).IsEmpty() {
					amlDict.CurrencyID = counterpartMoveLine.Currency().ID()
					amlDict.AmountCurrency = companyCurrency.WithNewContext(ctx).Compute(amlDict.Debit-amlDict.Credit, counterpartMoveLine.Currency(), true)
				}
				newAml := h.AccountMoveLine().NewSet(rs.Env()).WithContext("check_move_validity", false).Create(rs.ConvertAMLStructToData(amlDict))
				newAml.Union(counterpartMoveLine).Reconcile(h.AccountAccount().NewSet(rs.Env()), h.AccountJournal().NewSet(rs.Env()))
			}
			// Balance the move
			var stLineAmount float64
			for _, line := range move.Lines().Records() {
				stLineAmount -= line.Balance()
			}
			amlVals := rs.PrepareReconciliationMoveLine(move, stLineAmount)
			if payment.IsNotEmpty() {
				amlVals.SetPayment(payment)
			}
			h.AccountMoveLine().NewSet(rs.Env()).WithContext("check_move_validity", false).Create(amlVals)
			move.Post()
			// record the move name on the statement line to be able to retrieve it in case of unreconciliation
			rs.SetMoveName(move.Name())
			payment.SetPaymentReference(move.Name())
			counterPartMoves.AssertBalanced()
			return counterPartMoves
		})
}
