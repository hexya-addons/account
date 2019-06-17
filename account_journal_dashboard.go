// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"encoding/json"
	"fmt"

	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/strutils"
	"github.com/hexya-erp/hexya/src/views"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func init() {

	h.AccountJournal().AddFields(map[string]models.FieldDefinition{
		"KanbanDashboard": models.TextField{
			String:  "KanbanDashboard",
			Compute: h.AccountJournal().Methods().ComputeKanbanDashboard()},
		"KanbanDashboardGraph": models.TextField{
			String:  "KanbanDashboardGraph",
			Compute: h.AccountJournal().Methods().ComputeKanbanDashboardGraph()},
		"ShowOnDashboard": models.BooleanField{
			String:  "Show journal on dashboard",
			Help:    "Whether this journal should be displayed on the dashboard or not",
			Default: models.DefaultValue(true)},
	})

	h.AccountJournal().Methods().ComputeKanbanDashboard().DeclareMethod(
		`ComputeKanbanDashboard`,
		func(rs m.AccountJournalSet) m.AccountJournalData {
			str, err := json.Marshal(rs.GetJournalDashboardDatas())
			if err != nil {
				panic(rs.T("Could not marshall journal dashboard datas. error: %s", err)) //tovalid is panic needed? is message correct?
			}
			return h.AccountJournal().NewData().SetKanbanDashboard(string(str))
		})

	h.AccountJournal().Methods().ComputeKanbanDashboardGraph().DeclareMethod(
		`ComputeKanbanDashboardGraph`,
		func(rs m.AccountJournalSet) m.AccountJournalData {
			var str []byte
			var err error
			switch rs.Type() {
			case "sale", "purchase":
				str, err = json.Marshal(rs.GetBarGraphDatas())
			case "cash", "bank":
				str, err = json.Marshal(rs.GetLineGraphDatas())
			}
			if err != nil {
				// toalid error handling?
			}
			data := h.AccountJournal().NewData()
			if str != nil {
				data.SetKanbanDashboardGraph(string(str))
			}
			return data
		})

	h.AccountJournal().Methods().ToggleFavorite().DeclareMethod(
		`ToggleFavorite`,
		func(rs m.AccountJournalSet) bool {
			data := h.AccountJournal().NewData()
			data.SetShowOnDashboard(!rs.ShowOnDashboard())
			rs.Write(data)
			return false
		})

	h.AccountJournal().Methods().GetLineGraphDatas().DeclareMethod(
		`GetLineGraphDatas`,
		func(rs m.AccountJournalSet) []map[string]interface{} {
			today := dates.Today()
			lastMonth := today.AddDate(0, 0, -30)
			// Query to optimize loading of data for bank statement graphs
			// Return a list containing the latest bank statement balance per day for the
			// last 30 days for current journal
			var bankStmt []struct {
				Date       dates.Date
				BalanceEnd float64
			}
			{
				query := `
				SELECT a.date, a.balance_end
					FROM account_bank_statement AS a,
						(SELECT c.date, max(c.id) AS stmt_id
							FROM account_bank_statement AS c
							WHERE c.journal_id = ?
			                	AND c.date > ?
			                 	AND c.date <= ?
			                GROUP BY date, id
			                ORDER BY date, id) AS b
			        WHERE a.id = b.stmt_id;`
				rs.Env().Cr().Select(&bankStmt, query, rs.ID(), lastMonth, today)
			}
			query := q.AccountBankStatement().Journal().In(rs).And().Date().LowerOrEqual(lastMonth)
			lastBankStmt := h.AccountBankStatement().Search(rs.Env(), query).OrderBy("date desc", "id desc").Limit(1)
			startBalance := lastBankStmt.BalanceEnd()
			locale := rs.Env().Context().GetString("lang")
			if locale == "" {
				locale = "en_US"
			}
			showDate := lastMonth
			// get date in locale format
			name := ""      //format_date(show_date, 'd LLLL Y', locale=locale) //tovalid hexya format_date?
			shortName := "" //format_date(show_date, 'd MMM', locale=locale)
			data := []map[string]interface{}{{
				"x":    shortName,
				"y":    startBalance,
				"name": name,
			}}
			for _, stmt := range bankStmt {
				// fill the gap between last data and the new one
				numberDayToAdd := int(stmt.Date.Sub(showDate).Hours() / 24)
				lastBalance := data[len(data)-1]["y"].(float64)
				for i := 0; i <= numberDayToAdd; i++ {
					showDate = showDate.AddDate(0, 0, 1)
					// get date in locale format
					name := ""      //format_date(show_date, 'd LLLL Y', locale=locale) //tovalid hexya format_date?
					shortName := "" //format_date(show_date, 'd MMM', locale=locale)
					data = append(data, map[string]interface{}{
						"x":    shortName,
						"y":    lastBalance,
						"name": name,
					})
				}
				// add new stmt value
				data[len(data)-1]["y"] = stmt.BalanceEnd
			}
			//continue the graph if the last statement isn't today
			if showDate != today {
				numberDayToAdd := int(today.Sub(showDate).Hours() / 24)
				lastBalance := data[len(data)-1]["y"].(float64)
				for i := 0; i <= numberDayToAdd; i++ {
					showDate = showDate.AddDate(0, 0, 1)
					// get date in locale format
					name := ""      //format_date(show_date, 'd LLLL Y', locale=locale) //tovalid hexya format_date?
					shortName := "" //format_date(show_date, 'd MMM', locale=locale)
					data = append(data, map[string]interface{}{
						"x":    shortName,
						"y":    lastBalance,
						"name": name,
					})
				}
			}
			return []map[string]interface{}{{
				"values": data, //tovalid return array of interface containing array of interface. rly?
				"area":   true,
			}}
		})

	h.AccountJournal().Methods().GetBarGraphDatas().DeclareMethod(
		`GetBarGraphDatas`,
		func(rs m.AccountJournalSet) []map[string]interface{} {
			today := dates.Today()
			data := []map[string]interface{}{{
				"label": rs.T("Past"),
				"value": 0.0,
				"type":  "past",
			}}
			dayOfWeek := int(today.Weekday()) // int(format_datetime(today, 'e', locale=self._context.get('lang') or 'en_US')) // tovalid hexya format_datetime
			firstDayOfWeek := today.AddDate(0, 0, -dayOfWeek+1)
			for i := -1; i < 4; i++ {
				curData := map[string]interface{}{
					"value": 0.0,
					"type":  "future",
				}
				switch i {
				case 0:
					curData["label"] = rs.T("This Week")
				case 3:
					curData["label"] = rs.T("Future")
				default:
					if i < 0 {
						curData["type"] = "past"
					}
					startWeek := firstDayOfWeek.AddDate(0, 0, i*7)
					endWeek := startWeek.AddDate(0, 0, 6)
					if startWeek.Month() == endWeek.Month() {
						curData["label"] = fmt.Sprintf("%d-%d %s", startWeek.Day(), endWeek.Day(),
							"") //  format_date(end_week, 'MMM', locale=self._context.get('lang') or 'en_US') //tovalid hexya format_date
					} else {
						curData["label"] = fmt.Sprintf("%s-%s",
							"", /*format_date(start_week, 'd MMM', locale=self._context.get('lang') or 'en_US')*/
							"" /*format_date(end_week, 'd MMM', locale=self._context.get('lang') or 'en_US')*/)
					}
				}
				data = append(data, curData)
			}
			// Build SQL query to find amount aggregated by week
			query := ""
			selectSQLClause := `SELECT sum(residual_company_signed) as total, min(date) as aggr_date from account_invoice where journal_id = %s and state = 'open'`
			startDate := firstDayOfWeek.AddDate(0, 0, -7)
			for i := 0; i < 6; i++ {
				switch i {
				case 0:
					query = fmt.Sprintf(`%s ( %s and date < '%s' )`, query, selectSQLClause, startDate.String())
				case 5:
					query = fmt.Sprintf(`%s UNION ALL ( %s and date >= '%s' )`, query, selectSQLClause, startDate)
				default:
					nextDate := startDate.AddDate(0, 0, 7)
					query = fmt.Sprintf(`%s UNION ALL ( %s and date >= '%s' and date < '%s' )`, query, selectSQLClause, startDate, nextDate)
					startDate = nextDate
				}
			}
			var sqlResults []struct {
				Total    float64
				AggrDate dates.Date
			}
			id := rs.ID()
			rs.Env().Cr().Get(&sqlResults, query, id, id, id, id, id, id) // tovalid we really need named parameters
			for index, result := range sqlResults {
				if !result.AggrDate.IsZero() {
					data[index]["value"] = result.Total
				}
			}
			return []map[string]interface{}{{ //tovalid return array of interface containing array of interface. rly?
				"values": data,
			}}
		})

	h.AccountJournal().Methods().GetJournalDashboardDatas().DeclareMethod(
		`GetJournalDashboardDatas`,
		func(rs m.AccountJournalSet) map[string]interface{} {
			currency := h.Currency().Coalesce(rs.Currency(), rs.Company().Currency())
			var numberToReconcile int64
			var lastBalance float64
			var accountSum float64
			var title string
			var numberDraft int
			var numberWaiting int
			var numberLate int
			var sumDraft float64
			var sumWaiting float64
			var sumLate float64
			switch rs.Type() {
			case "bank", "cash":
				lastBankStmt := h.AccountBankStatement().Search(rs.Env(), q.AccountBankStatement().Journal().In(rs)).OrderBy("date desc", "id desc").Limit(1)
				lastBalance = lastBankStmt.BalanceEnd()
				// Get the number of items to reconcile for that bank journal
				var sqlReturn int64
				rs.Env().Cr().Get(&sqlReturn, `SELECT COUNT(DISTINCT(statement_line_id))
							FROM account_move where statement_line_id
							IN (SELECT line.id
								FROM account_bank_statement_line AS line
								LEFT JOIN account_bank_statement AS st
								ON line.statement_id = st.id
								WHERE st.journal_id IN (?) and st.state = 'open')`, rs.Ids())
				alreadyReconciled := sqlReturn
				rs.Env().Cr().Get(&sqlReturn, `SELECT COUNT(line.id)
								FROM account_bank_statement_line AS line
								LEFT JOIN account_bank_statement AS st
								ON line.statement_id = st.id
								WHERE st.journal_id IN (?) and st.state = 'open'`, rs.Ids())
				allLines := sqlReturn
				numberToReconcile = allLines - alreadyReconciled
				// optimization to read sum of balance from account_move_line
				accountIds := rs.DefaultDebitAccount().Union(rs.DefaultCreditAccount())
				if accountIds.IsNotEmpty() {
					amountField := "amount_currency"
					if rs.Currency().IsEmpty() || rs.Currency().Equals(rs.Company().Currency()) {
						amountField = "balance"
					}
					query := fmt.Sprintf(`	SELECT coalesce(sum(aml.%s), 0) 
													FROM account_move_line AS aml 
    												LEFT JOIN account_move am ON aml.move_id = am.id 
													WHERE aml.account_id IN (?) AND am.date <= ?
													LIMIT 1;`, amountField)
					var sqlSum int64
					rs.Env().Cr().Get(&sqlSum, query, accountIds.Ids(), dates.Today())
					if sqlSum != 0 {
						accountSum = float64(sqlSum)
					}
				}
			//TODO need to check if all invoices are in the same currency than the journal!!!!
			case "sale", "purchase":
				title = rs.T(`Invoices owed to you`)
				if rs.Type() == "purchase" {
					title = rs.T(`Bills to pay`)
				}
				// optimization to find total and sum of invoice that are in draft, open state
				query := q.AccountInvoice().Journal().Equals(rs).And().State().NotIn([]string{"paid", "cancel"})
				queryResults := h.AccountInvoice().Search(rs.Env(), query)
				for _, result := range queryResults.Records() {
					factor := 1.0
					if strutils.IsIn(result.Type(), "in_refund", "out_refund") {
						factor = -1.0
					}
					if strutils.IsIn(result.State(), "draft", "proforma", "proforma2") {
						numberDraft += 1
						sumDraft += result.Currency().Compute(result.AmountTotal(), currency, true) * factor
					} else if result.State() == "open" {
						numberWaiting += 1
						sumWaiting += result.Currency().Compute(result.AmountTotal(), currency, true) * factor
					}
				}
				today := dates.Today()
				query = q.AccountInvoice().Journal().Equals(rs).And().Date().Lower(today)
				lateQueryResults := h.AccountInvoice().Search(rs.Env(), query)
				for _, result := range lateQueryResults.Records() {
					factor := 1.0
					if strutils.IsIn(result.Type(), "in_refund", "out_refund") {
						factor = -1.0
					}
					numberLate += 1
					sumLate += result.Currency().Compute(result.AmountTotal(), currency, true) * factor
				}
			}
			ret := map[string]interface{}{
				`number_to_reconcile`:    numberToReconcile,
				`account_balance`:        FormatLang(rs.Env(), accountSum, currency),
				`last_balance`:           FormatLang(rs.Env(), lastBalance, currency),
				`number_draft`:           numberDraft,
				`number_waiting`:         numberWaiting,
				`number_late`:            numberLate,
				`sum_draft`:              FormatLang(rs.Env(), sumDraft, currency),
				`sum_waiting`:            FormatLang(rs.Env(), sumWaiting, currency),
				`sum_late`:               FormatLang(rs.Env(), sumLate, currency),
				`currency_id`:            currency,
				`bank_statements_source`: rs.BankStatementsSource(),
				`title`:                  title,
			}
			if val := lastBalance - accountSum; val != 0.0 {
				ret["difference"] = FormatLang(rs.Env(), val, currency)
			}
			return ret
		})

	h.AccountJournal().Methods().ActionCreateNew().DeclareMethod(
		`ActionCreateNew`,
		//TODO shorten me
		func(rs m.AccountJournalSet) *actions.Action {
			ctx := rs.Env().Context().Copy().WithKey("default_journal", rs)
			view := views.MakeViewRef("account_view_move_form")
			model := "AccountMove"
			switch rs.Type() {
			case "sale":
				ctx = ctx.
					WithKey("journal_type", rs.Type()).
					WithKey("default_type", "out_invoice").
					WithKey("type", "out_invoice")
				if ctx.GetBool("refund") {
					ctx = ctx.
						WithKey("default_type", "out_refund").
						WithKey("type", "out_refund")
				}
				view = views.MakeViewRef("account_invoice_form")
				model = "AccountInvoice"
			case "purchase":
				ctx = ctx.
					WithKey("journal_type", rs.Type()).
					WithKey("default_type", "in_invoice").
					WithKey("type", "in_invoice")
				if ctx.GetBool("refund") {
					ctx = ctx.
						WithKey("default_type", "in_refund").
						WithKey("type", "in_refund")
				}
				view = views.MakeViewRef("account_invoice_supplier_form")
				model = "AccountInvoice"
			}
			return &actions.Action{
				Name:     rs.T(`Create invoice/bill`),
				Type:     actions.ActionActWindow,
				ViewMode: "form",
				Model:    model,
				View:     view,
				Context:  ctx,
			}
		})

	h.AccountJournal().Methods().CreateCashStatement().DeclareMethod(
		`CreateCashStatement`,
		func(rs m.AccountJournalSet) *actions.Action {
			ctx := rs.Env().Context().Copy().
				WithKey("journal", rs).
				WithKey("default_journal", rs).
				WithKey("default_journal_type", "cash")
			return &actions.Action{
				Name:     rs.T("Create cash statement"),
				Type:     actions.ActionActWindow,
				ViewMode: "form",
				Model:    "AccountBankStatement",
				Context:  ctx,
			}
		})

	h.AccountJournal().Methods().ActionOpenReconcile().DeclareMethod(
		`ActionOpenReconcile`,
		func(rs m.AccountJournalSet) *actions.Action {
			out := actions.Action{
				Type: actions.ActionClient,
			}
			ctx := types.NewContext().WithKey("company_ids", rs.Company().Ids())
			if strutils.IsIn(rs.Type(), "bank", "cash") {
				// Open reconciliation view for bank statements belonging to this journal
				bankStmt := h.AccountBankStatement().Search(rs.Env(), q.AccountBankStatement().Journal().In(rs))
				out.Tag = "bank_statement_reconciliation_view"
				out.Context = ctx.WithKey("statement_ids", bankStmt.Ids())
			} else {
				// Open reconciliation view for customers/suppliers
				ctx = ctx.WithKey("show_mode_selector", false)
				switch rs.Type() {
				case "sale":
					ctx = ctx.WithKey("mode", "customers")
				case "putchase":
					ctx = ctx.WithKey("mode", "suppliers")
				}
				out.Tag = "manual_reconciliation_view"
				out.Context = ctx
			}
			return &out
		})

	h.AccountJournal().Methods().OpenAction().DeclareMethod(
		`OpenAction return action based on type for related journals`,
		func(rs m.AccountJournalSet) *actions.Action {
			actionName := rs.Env().Context().GetString("action_name")
			if actionName == "" {
				switch rs.Type() {
				case "bank":
					actionName = "action_bank_statement_tree"
				case "cash":
					actionName = "action_view_bank_statement_tree"
				case "sale":
					actionName = "action_invoice_tree1"
				case "purchase":
					actionName = "action_invoice_tree2"
				default:
					actionName = "action_move_journal_line"
				}
			}

			invoiceType := map[string]string{
				"sale":           "out_invoice",
				"purchase":       "in_invoice",
				"salerefund":     "out_refund",
				"purchaserefund": "in_refund",
				"bank":           "bank",
				"cash":           "cash",
				"general":        "general",
			}[rs.Type()+rs.Env().Context().GetString("invoice_type")]

			ctx := rs.Env().Context().Copy().
				Delete("group_by").
				WithKey("journal_type", rs.Type()).
				WithKey("default_journal_id", rs.ID()).
				WithKey("search_default_journal_id", rs.ID()).
				WithKey("default_type", invoiceType).
				WithKey("type", invoiceType)

			action := actions.Registry.GetById("account_" + actionName)
			action.Context = ctx
			action.Domain = rs.Env().Context().GetString("use_domain")
			if action.Domain == "" {
				action.Domain = "[]"
			}
			if strutils.IsIn(actionName, "action_bank_statement_tree", "action_view_bank_statement_tree") {
				action.Views = nil
				action.View = views.ViewRef{}
			}
			return action
		})

	h.AccountJournal().Methods().OpenSpendMoney().DeclareMethod(
		`OpenSpendMoney`,
		func(rs m.AccountJournalSet) *actions.Action {
			return rs.OpenPaymentsAction("outbound")
		})

	h.AccountJournal().Methods().OpenCollectMoney().DeclareMethod(
		`OpenCollectMoney`,
		func(rs m.AccountJournalSet) *actions.Action {
			return rs.OpenPaymentsAction("inbound")
		})

	h.AccountJournal().Methods().OpenTransferMoney().DeclareMethod(
		`OpenTransferMoney`,
		func(rs m.AccountJournalSet) *actions.Action {
			return rs.OpenPaymentsAction("transfer")
		})

	h.AccountJournal().Methods().OpenPaymentsAction().DeclareMethod(
		`OpenPaymentsAction`,
		func(rs m.AccountJournalSet, paymentType string) *actions.Action {
			ctx := rs.Env().Context().Copy().
				WithKey("default_payment_type", paymentType).
				WithKey("default_journal_id", rs.ID()).
				Delete("group_by")
			/*
			  action_rec = self.env['ir.model.data'].xmlid_to_object('account.action_account_payments') //tovalid ir.model.data
			  if action_rec:
			      action = action_rec.read([])[0]
			      action['context'] = ctx
			      action['domain'] = [('journal_id','=',self.id),('payment_type','=',payment_type)]
			      return action
			*/
			// FIXME
			fmt.Println(ctx)
			return new(actions.Action)
		})

	h.AccountJournal().Methods().OpenActionWithContext().DeclareMethod(
		`OpenActionWithContext`,
		func(rs m.AccountJournalSet) *actions.Action {
			ctx := rs.Env().Context().
				WithKey("default_journal_id", rs.ID()).
				Delete("group_by")
			actionName := rs.Env().Context().GetString("action_name")
			if actionName == "" {
				return new(actions.Action)
			}
			if ctx.GetBool("search_default_journal") {
				ctx = ctx.WithKey("search_default_journal_id", rs.ID())
			}
			// ir_model_obj = self.env['ir.model.data'] //tovalid ir.model.data
			// model, action_id = ir_model_obj.get_object_reference('account', action_name)
			// [action] = self.env[model].browse(action_id).read()
			var action actions.Action
			action.Context = ctx
			if ctx.GetBool("use_domain") {
				action.Domain = "['|', ('journal_id', '=', self.id), ('journal_id', '=', False)]"
				action.Name += "for journal" + rs.Name()
			}
			return &action
		})

	h.AccountJournal().Methods().CreateBankStatement().DeclareMethod(
		`CreateBankStatement return action to create a bank statements. This button should be called only on journals with type =='bank'`,
		func(rs m.AccountJournalSet) *actions.Action {
			rs.SetBankStatementsSource("manual")
			action := actions.Registry.GetById("account_action_bank_statement_tree")
			action.Views = []views.ViewTuple{{"", "form"}}
			action.Context = types.NewContext().WithKey("default_journal_id", rs.ID())
			return action
		})

}
