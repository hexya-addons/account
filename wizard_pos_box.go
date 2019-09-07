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

	h.CashBox().DeclareMixinModel()
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
		func(rs m.CashBoxSet) {
			//activeModel := rs.Env().Context().GetString("active_model")
			// in odoo, the context value associated with "active_model" is used
			// since hexya is type safe, its not the case
			// this may need to be changed
			activeIds := rs.Env().Context().GetIntegerSlice("active_ids")
			records := h.AccountBankStatement().Browse(rs.Env(), activeIds)
			rs.RunPrivate(records)
		})

	h.CashBox().Methods().RunPrivate().DeclareMethod(
		`RunPrivate`,
		func(rs m.CashBoxSet, records m.AccountBankStatementSet) {
			for _, box := range rs.Records() {
				for _, record := range records.Records() {
					if record.Journal().IsEmpty() {
						panic(rs.T(`Please check that the field 'Journal' is set on the Bank Statement`))
					} else if record.Journal().Company().TransferAccount().IsEmpty() {
						panic(rs.T(`Please check that the field 'Transfer Account' is set on the company.`))
					}
					box.CreateBankStatementLine(record)
				}
			}
		})

	h.CashBox().Methods().CreateBankStatementLine().DeclareMethod(
		`CreateBankStatementLine`,
		func(rs m.CashBoxSet, record m.AccountBankStatementSet) bool {
			rs.EnsureOne()
			if record.State() == "confirm" {
				panic(rs.T(`You cannot put/take money in/out for a bank statement which is closed.`))
			}
			line := h.AccountBankStatementLine().Create(rs.Env(), rs.CalculateValuesForStatementLine(record))
			return record.Write(h.AccountBankStatement().NewData().SetLines(line))
		})

	h.CashBox().Methods().CalculateValuesForStatementLine().DeclareMethod(
		`CalculateValuesForStatementLine`,
		func(rs m.CashBoxSet, record m.AccountBankStatementSet) m.AccountBankStatementLineData {
			return h.AccountBankStatementLine().NewData()
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
