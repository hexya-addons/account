// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
)

func init() {

	h.AccountPrintJournal().DeclareTransientModel()
	h.AccountPrintJournal().InheritModel(h.AccountCommonJournalReport())

	h.AccountPrintJournal().AddFields(map[string]models.FieldDefinition{
		"SortSelection": models.SelectionField{
			String: "Entries Sorted by",
			Selection: types.Selection{
				"date":      "Date",
				"move_name": "Journal Entry Number"},
			Required: true,
			Default:  models.DefaultValue("move_name")},
	})
	h.AccountPrintJournal().Methods().PrintReport().DeclareMethod(
		`PrintReport`,
		func(rs m.AccountCommonJournalReportSet, data map[string]interface{}) map[string]interface{} {
			/*def _print_report(self, data):
			  data = self.pre_print_report(data)
			  data['form'].update({'sort_selection': self.sort_selection})
			  return self.env['report'].with_context(landscape=True).get_action(self, 'account.report_journal', data=data)
			*/
			return nil
		})

}
