// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
)

func init() {

	h.CashBox().DeclareTransientModel()
	h.CashBox().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:   "Name",
			Required: true},
		"Amount": models.FloatField{
			String: "Amount",
			Digits: nbutils.Digits{
				Precision: 0,
				Scale:     0},
			Required: true},
	})
	h.CashBox().Methods().Run().DeclareMethod(
		`Run`,
		func(rs m.CashBoxSet) interface{} {
			active_model := rs.Env().Context().GetString("active_model")
			if active_model == "" {
				return rs.RunPrivate(&models.RecordCollection{})
			}
			activeIds := rs.Env().Context().GetIntegerSlice("active_ids")
			collection := rs.Env().Pool(active_model)
			records := collection.Search(collection.Model().Field("ID").In(activeIds))
			return rs.RunPrivate(records)
		})

	h.CashBox().Methods().RunPrivate().DeclareMethod(
		`RunPrivate`,
		func(rs m.CashBoxSet, records models.RecordSet) interface{} {
			//@api.multi
			/*def _run(self, records):
			  for box in self:
			      for record in records:
			          if not record.journal_id:
			              raise UserError(_("Please check that the field 'Journal' is set on the Bank Statement"))
			          if not record.journal_id.company_id.transfer_account_id:
			              raise UserError(_("Please check that the field 'Transfer Account' is set on the company."))
			          box._create_bank_statement_line(record)
			  return {}

			*/
		})
	h.CashBox().Methods().CreateBankStatementLine().DeclareMethod(
		`CreateBankStatementLine`,
		func(rs m.CashBoxSet, args struct {
			Record interface{}
		}) {
			//@api.one
			/*def _create_bank_statement_line(self, record):
			  if record.state == 'confirm':
			      raise UserError(_("You cannot put/take money in/out for a bank statement which is closed."))
			  values = self._calculate_values_for_statement_line(record)
			  return record.write({'line_ids': [(0, False, values)]})


			*/
		})

	h.CashBoxIn().DeclareTransientModel()
	h.CashBoxIn().AddFields(map[string]models.FieldDefinition{
		"Ref": models.CharField{
			String: "Reference')"},
	})
	h.CashBoxIn().InheritModel(h.CashBox())
	h.CashBoxIn().Methods().CalculateValuesForStatementLine().DeclareMethod(
		`CalculateValuesForStatementLine`,
		func(rs m.CashBoxInSet, record m.AccountBankStatementSet) m.AccountBankStatementLineData {
			if record.Journal().Company().TransferAccount().IsEmpty() {
				panic(rs.T(`You should have defined an 'Internal Transfer Account' in your cash register's journal!`))
			}
			return h.AccountBankStatementLine().NewData().
				SetDate(record.Date()).
				SetStatement(record).
				SetJournal(record.Journal()).
				SetAmount(rs.Amount()).
				SetAccount(record.Journal().Company().TransferAccount()).
				SetRef(rs.Ref()).
				SetName(rs.Name())
		})

	h.CashBoxOut().DeclareTransientModel()
	h.CashBoxOut().InheritModel(h.CashBox())
	h.CashBoxOut().Methods().CalculateValuesForStatementLine().DeclareMethod(
		`CalculateValuesForStatementLine`,
		func(rs m.CashBoxOutSet, record m.AccountBankStatementSet) m.AccountBankStatementLineData {
			if record.Journal().Company().TransferAccount().IsEmpty() {
				panic(rs.T(`You should have defined an 'Internal Transfer Account' in your cash register's journal!`))
			}
			data := h.AccountBankStatementLine().NewData().
				SetDate(record.Date()).
				SetStatement(record).
				SetJournal(record.Journal()).
				SetAmount(rs.Amount()).
				SetAccount(record.Journal().Company().TransferAccount()).
				SetName(rs.Name())
			if rs.Amount() > 0 {
				data.SetAmount(-rs.Amount())
			}
			return data
		})

}
