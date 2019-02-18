// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
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
			/*[currency_field 'company_currency_id']*/
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
		func(rs h.TaxAdjustmentsWizardSet) h.AccountMoveSet {
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
		func(rs h.TaxAdjustmentsWizardSet) {
			//@api.multi
			/*def create_move(self):
			  #create the adjustment move
			  move_id = self._create_move()
			  #return an action showing the created move
			  action = self.env.ref(self.env.context.get('action', 'account.action_move_line_form'))
			  result = action.read()[0]
			  result['views'] = [(False, 'form')]
			  result['res_id'] = move_id
			  return result
			*/
		})

}
