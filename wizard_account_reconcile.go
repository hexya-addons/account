// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
	"github.com/hexya-erp/hexya/src/views"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func init() {

	h.AccountMoveLineReconcile().DeclareTransientModel()
	h.AccountMoveLineReconcile().AddFields(map[string]models.FieldDefinition{
		"TransNbr": models.IntegerField{
			String:   "# of Transaction",
			ReadOnly: true},
		"Credit": models.FloatField{
			String:   "Credit amount",
			ReadOnly: true,
			Digits:   nbutils.Digits{Precision: 0, Scale: 0}},
		"Debit": models.FloatField{
			String:   "Debit amount",
			ReadOnly: true,
			Digits:   nbutils.Digits{Precision: 0, Scale: 0}},
		"Writeoff": models.FloatField{
			String:   "Write-off amount",
			ReadOnly: true,
			Digits:   nbutils.Digits{Precision: 0, Scale: 0}},
		"Company": models.Many2OneField{
			String:        "Company",
			RelationModel: h.Company(),
			Required:      true,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser().Company()
			}},
	})
	h.AccountMoveLineReconcile().Methods().DefaultGet().Extend(
		`DefaultGet`,
		func(rs m.AccountMoveLineReconcileSet) models.FieldMap {
			res := rs.Super().DefaultGet()
			data := rs.TransRecGet()
			res["trans_nbr"] = data["trans_nbr"]
			res["credit"] = data["credit"]
			res["debit"] = data["debit"]
			res["writeoff"] = data["writeoff"]
			return res
		})

	h.AccountMoveLineReconcile().Methods().TransRecGet().DeclareMethod(
		`TransRecGet`,
		func(rs m.AccountMoveLineReconcileSet) map[string]interface{} {
			var credit float64
			var debit float64

			lines := h.AccountMoveLine().Browse(rs.Env(), rs.Env().Context().GetIntegerSlice("active_ids"))
			for _, line := range lines.Records() {
				if line.FullReconcile().IsEmpty() {
					credit += line.Credit()
					debit += line.Debit()
				}
			}
			precision := nbutils.Digits{
				Scale: int8(h.User().NewSet(rs.Env()).CurrentUser().Company().Currency().DecimalPlaces()),
			}.ToPrecision()
			writeOff := nbutils.Round(debit-credit, precision)
			credit = nbutils.Round(credit, precision)
			debit = nbutils.Round(debit, precision)
			return map[string]interface{}{
				"trans_nbr": lines.Len(),
				"credit":    credit,
				"debit":     debit,
				"writeoff":  writeOff,
			}
		})

	h.AccountMoveLineReconcile().Methods().TransRecAddendumWriteoff().DeclareMethod(
		`TransRecAddendumWriteoff`,
		func(rs m.AccountMoveLineReconcileSet) *actions.Action {
			return h.AccountMoveLineReconcileWriteoff().NewSet(rs.Env()).TransRecAddendum()
		})

	h.AccountMoveLineReconcile().Methods().TransRecReconcilePartialReconcile().DeclareMethod(
		`TransRecReconcilePartialReconcile`,
		func(rs m.AccountMoveLineReconcileSet) *actions.Action {
			return h.AccountMoveLineReconcileWriteoff().NewSet(rs.Env()).TransRecReconcilePartial()
		})

	h.AccountMoveLineReconcile().Methods().TransRecReconcileFull().DeclareMethod(
		`TransRecReconcileFull`,
		func(rs m.AccountMoveLineReconcileSet) *actions.Action {
			moveLines := h.AccountMoveLine().Browse(rs.Env(), rs.Env().Context().GetIntegerSlice("active_ids"))
			currency := h.Currency().NewSet(rs.Env())
			for _, aml := range moveLines.Records() {
				if currency.IsEmpty() && aml.Currency().IsNotEmpty() {
					currency = aml.Currency()
				} else if aml.Currency().IsNotEmpty() && aml.Currency().Equals(currency) {
					continue
				}
				panic(rs.T(`Operation not allowed. You can only reconcile entries that share the same secondary currency or that don\'t have one. Edit your journal items or make another selection before proceeding any further.`))
			}
			// Don't consider entrires that are already reconciled
			moveLinesFiltered := moveLines.Filtered(func(set m.AccountMoveLineSet) bool {
				return !set.Reconciled()
			})
			// Because we are making a full reconcilition in batch, we need to consider use cases as defined in the test test_manual_reconcile_wizard_opw678153
			// So we force the reconciliation in company currency only at first
			moveLinesFiltered.
				WithContext("skip_full_reconcile_check", "amount_currency_excluded").
				WithContext("manual_full_reconcile_currency_id", currency.ID()).
				Reconcile(h.AccountAccount().NewSet(rs.Env()), h.AccountJournal().NewSet(rs.Env()))
			// then in second pass the amounts in secondary currency, only if some lines are still not fully reconciled
			moveLinesFiltered = moveLines.Filtered(func(set m.AccountMoveLineSet) bool {
				return !set.Reconciled()
			})
			if moveLinesFiltered.IsNotEmpty() {
				moveLinesFiltered.
					WithContext("skip_full_reconcile_check", "amount_currency_only").
					WithContext("manual_full_reconcile_currency_id", currency.ID()).
					Reconcile(h.AccountAccount().NewSet(rs.Env()), h.AccountJournal().NewSet(rs.Env()))
			}
			moveLines.ComputeFullAfterBatchReconcile()
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

	h.AccountMoveLineReconcileWriteoff().DeclareTransientModel()
	h.AccountMoveLineReconcileWriteoff().AddFields(map[string]models.FieldDefinition{
		"Journal": models.Many2OneField{
			String:        "Write-Off Journal",
			RelationModel: h.AccountJournal(),
			JSON:          "journal_id",
			Required:      true},
		"WriteoffAcc": models.Many2OneField{
			String:        "Write-Off account",
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			JSON:          "writeoff_acc_id",
			Required:      true},
		"DateP": models.DateField{
			String:  "Date",
			Default: models.DefaultValue(dates.Today())},
		"Comment": models.CharField{
			String:   "Comment",
			Required: true,
			Default:  models.DefaultValue("Write-off")},
		"Analytic": models.Many2OneField{
			String: "Analytic Account", RelationModel: h.AccountAnalyticAccount(), JSON: "analytic_id"},
	})
	h.AccountMoveLineReconcileWriteoff().Methods().TransRecAddendum().DeclareMethod(
		`TransRecAddendum`,
		func(rs m.AccountMoveLineReconcileWriteoffSet) *actions.Action {
			view := views.Registry.GetByID("account.account_move_line_reconcile_writeoff")
			return &actions.Action{
				Name:     rs.T("Reconcile Writeoff"),
				Context:  rs.Env().Context(),
				ViewMode: "form",
				Model:    "account.move.line.reconcile.writeoff",
				Views:    []views.ViewTuple{{view.ID, "form"}},
				Type:     actions.ActionActWindow,
				Target:   "new",
			}
		})
	h.AccountMoveLineReconcileWriteoff().Methods().TransRecReconcilePartial().DeclareMethod(
		`TransRecReconcilePartial`,
		func(rs m.AccountMoveLineReconcileWriteoffSet) *actions.Action {
			//@api.multi
			h.AccountMoveLine().
				Browse(rs.Env(), rs.Env().Context().GetIntegerSlice("active_ids")).
				Reconcile(h.AccountAccount().NewSet(rs.Env()), h.AccountJournal().NewSet(rs.Env()))
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})
	h.AccountMoveLineReconcileWriteoff().Methods().TransRecReconcile().DeclareMethod(
		`TransRecReconcile`,
		func(rs m.AccountMoveLineReconcileWriteoffSet) *actions.Action {
			context := rs.Env().Context().
				WithKey("date_p", rs.DateP()).
				WithKey("comment", rs.Comment())
			if rs.Analytic().IsNotEmpty() {
				context = context.WithKey("analytic_id", rs.Analytic().ID())
			}
			moveLines := h.AccountMoveLine().Browse(rs.Env(), context.GetIntegerSlice("active_ids"))
			currency := h.Currency().NewSet(rs.Env())
			for _, aml := range moveLines.Records() {
				if currency.IsEmpty() && aml.Currency().IsNotEmpty() {
					currency = aml.Currency()
				} else if aml.Currency().IsNotEmpty() && aml.Currency().Equals(currency) {
					continue
				}
				panic(rs.T(`Operation not allowed. You can only reconcile entries that share the same secondary currency or that don\'t have one. Edit your journal items or make another selection before proceeding any further.`))
			}

			// Don't consider entrires that are already reconciled
			moveLinesFiltered := moveLines.Filtered(func(set m.AccountMoveLineSet) bool {
				return !set.Reconciled()
			})
			// Because we are making a full reconcilition in batch, we need to consider use cases as defined in the test test_manual_reconcile_wizard_opw678153
			// So we force the reconciliation in company currency only at first,
			context = context.
				WithKey("skip_full_reconcile_check", "amount_currency_excluded").
				WithKey("manual_full_reconcile_currency_id", currency.ID())
			writeoff := moveLinesFiltered.WithNewContext(context).Reconcile(rs.WriteoffAcc(), rs.Journal())
			// then in second pass the amounts in secondary currency, only if some lines are still not fully reconciled
			moveLinesFiltered = moveLines.Filtered(func(set m.AccountMoveLineSet) bool {
				return !set.Reconciled()
			})
			if moveLinesFiltered.IsNotEmpty() {
				moveLinesFiltered.
					WithContext("skip_full_reconcile_check", "amount_currency_only").
					WithContext("manual_full_reconcile_currency_id", currency.ID()).
					Reconcile(h.AccountAccount().NewSet(rs.Env()), h.AccountJournal().NewSet(rs.Env()))
			}
			if writeoff.IsNotEmpty() {
				moveLines = moveLines.Union(writeoff)
			}
			moveLines.ComputeFullAfterBatchReconcile()
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

}
