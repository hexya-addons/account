// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
)

func init() {

	h.AccountMoveReversal().DeclareTransientModel()
	h.AccountMoveReversal().AddFields(map[string]models.FieldDefinition{
		"Date": models.DateField{
			String: "Date" /*[string 'Reversal date']*/ /*[ default fields.Date.context_today]*/ /*[ required True]*/},
		"Journal": models.Many2OneField{
			String:        "Use Specific Journal",
			RelationModel: h.AccountJournal(),
			JSON:          "journal_id", /*['account.journal']*/
			Help:          "If empty, uses the journal of the journal entry to be reversed." /*[ uses the journal of the journal entry to be reversed.']*/},
	})
	h.AccountMoveReversal().Methods().ReverseMoves().DeclareMethod(
		`ReverseMoves`,
		func(rs m.AccountMoveReversalSet) *actions.Action {
			acMoveIDs := rs.Env().Context().GetIntegerSlice("active_ids")
			res := h.AccountMove().Browse(rs.Env(), acMoveIDs).ReverseMoves(rs.Date(), rs.Journal())
			if res.IsNotEmpty() {
				return &actions.Action{
					Name:     rs.T(`Reverse Moves`),
					Type:     actions.ActionActWindow,
					ViewMode: "tree,form",
					Model:    "AccountMove",
					Domain:   "[('id', 'in', res)]",
				}
			}
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

}
