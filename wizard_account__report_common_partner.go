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

	h.AccountCommonPartnerReport().DeclareMixinModel()
	h.AccountCommonPartnerReport().InheritModel(h.AccountCommonReport())

	h.AccountCommonPartnerReport().AddFields(map[string]models.FieldDefinition{
		"ResultSelection": models.SelectionField{
			Selection: types.Selection{
				"customer":          "Receivable Accounts",
				"supplier":          "Payable Accounts",
				"customer_supplier": "Receivable and Payable Accounts"}},
	})
	h.AccountCommonPartnerReport().Methods().PrePrintReport().DeclareMethod(
		`PrePrintReport`,
		func(rs m.AccountCommonPartnerReportSet, args struct {
			Data interface{}
		}) {
			/*def pre_print_report(self, data):
			  data['form'].update(self.read(['result_selection'])[0])
			  return data
			*/
		})

}
