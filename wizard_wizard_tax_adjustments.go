// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/views"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func init() {

	h.TaxAdjustmentsWizard().DeclareTransientModel()

	h.TaxAdjustmentsWizard().AddFields(map[string]models.FieldDefinition{
		"Reason": models.CharField{
			String:   "Justification",
			Required: true},
		"Journal": models.Many2OneField{
			RelationModel: h.AccountJournal(),
			Required:      true,
			Default: func(env models.Environment) interface{} {
				return h.AccountJournal().Search(env, q.AccountJournal().Type().Equals("general")).Limit(1)
			},
			Filter: q.AccountJournal().Type().Equals("general")},
		"Date": models.DateField{
			Required: true,
			Default: func(env models.Environment) interface{} {
				return dates.Today()
			}},
		"DebitAccount": models.Many2OneField{
			RelationModel: h.AccountAccount(),
			Required:      true,
			Filter:        q.AccountAccount().Deprecated().Equals(false)},
		"CreditAccount": models.Many2OneField{
			RelationModel: h.AccountAccount(),
			Required:      true,
			Filter:        q.AccountAccount().Deprecated().Equals(false)},
		"Amount": models.FloatField{
			Required: true},
		"CompanyCurrency": models.Many2OneField{
			RelationModel: h.Currency(),
			ReadOnly:      true,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser().Company()
			}},
		"Tax": models.Many2OneField{
			String:        "Adjustment Tax",
			RelationModel: h.AccountTax(),
			OnDelete:      models.Restrict,
			Required:      true,
			Filter:        q.AccountTax().TypeTaxUse().Equals("none").And().TaxAdjustment().Equals(true)},
	})

	h.TaxAdjustmentsWizard().Methods().CreateMovePrivate().DeclareMethod(
		`CreateMovePrivate`,
		func(rs m.TaxAdjustmentsWizardSet) m.AccountMoveSet {
			debitData := h.AccountMoveLine().NewData().
				SetName(rs.Reason()).
				SetDebit(rs.Amount()).
				SetCredit(0.0).
				SetAccount(rs.DebitAccount()).
				SetTaxLine(rs.Tax())
			creditData := h.AccountMoveLine().NewData().
				SetName(rs.Reason()).
				SetDebit(0.0).
				SetCredit(rs.Amount()).
				SetAccount(rs.CreditAccount()).
				SetTaxLine(rs.Tax())
			data := h.AccountMove().NewData().
				SetJournal(rs.Journal()).
				SetDate(rs.Date()).
				SetState("draft").
				SetLines(h.AccountMoveLine().Create(rs.Env(), debitData).Union(
					h.AccountMoveLine().Create(rs.Env(), creditData)))
			move := h.AccountMove().Create(rs.Env(), data)
			move.Post()
			return move
		})

	h.TaxAdjustmentsWizard().Methods().CreateMove().DeclareMethod(
		`CreateMove`,
		func(rs m.TaxAdjustmentsWizardSet) *actions.Action {
			// create the adjustment move
			move := rs.CreateMovePrivate()
			// return an action showing the created move
			actionId := rs.Env().Context().GetString("action")
			if actionId == "" {
				actionId = "account.action_move_line_from"
			}
			action := actions.Registry.GetById(actionId)
			action.Views = []views.ViewTuple{{"", "form"}}
			action.ResID = move.ID()
			return action
		})

}
