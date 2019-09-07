// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func init() {

	h.AccountingReport().DeclareTransientModel()
	h.AccountingReport().InheritModel(h.AccountCommonReport())

	accountingReportAccountReportDefaultFunc := func(env models.Environment) interface{} {
		id := env.Context().GetInteger("active_id")
		if id == 0 {
			return h.AccountFinancialReport().NewSet(env)
		}
		menu := ""
		reports := h.AccountFinancialReport().Search(env, q.AccountFinancialReport().Name().Contains(menu))
		if reports.IsNotEmpty() {
			return reports.Records()[0]
		}
		return reports
	}

	h.AccountingReport().AddFields(map[string]models.FieldDefinition{
		"EnableFilter": models.BooleanField{
			String: "Enable Comparison"},
		"AccountReport": models.Many2OneField{
			String:        "Account Reports",
			RelationModel: h.AccountFinancialReport(),
			JSON:          "account_report_id",
			Required:      true,
			Default:       accountingReportAccountReportDefaultFunc},
		"LabelFilter": models.CharField{
			String: "Column Label",
			Help:   "This label will be displayed on report to show the balance computed for the given comparison filter."},
		"FilterCmp": models.SelectionField{
			String: "Filter by",
			Selection: types.Selection{
				"filter_no":   "No Filters",
				"filter_date": "Date"},
			Required: true,
			Default:  models.DefaultValue("filter_no")},
		"DateFromCmp": models.DateField{
			String: "Start Date"},
		"DateToCmp": models.DateField{
			String: "End Date"},
		"DebitCredit": models.BooleanField{
			String: "Display Debit/Credit Columns",
			Help:   "This option allows you to get more details about the way your balances are computed. Because it is space consuming, we do not allow to use it while doing a comparison."},
	})
	h.AccountingReport().Methods().BuildComparisonContext().DeclareMethod(
		`BuildComparisonContext`,
		func(rs m.AccountingReportSet, args struct {
			Data interface{}
		}) {
			/*def _build_comparison_context(self, data):
			  result = {}
			  result['journal_ids'] = 'journal_ids' in data['form'] and data['form']['journal_ids'] or False
			  result['state'] = 'target_move' in data['form'] and data['form']['target_move'] or ''
			  if data['form']['filter_cmp'] == 'filter_date':
			      result['date_from'] = data['form']['date_from_cmp']
			      result['date_to'] = data['form']['date_to_cmp']
			      result['strict_range'] = True
			  return result

			*/
		})
	h.AccountingReport().Methods().CheckReport().DeclareMethod(
		`CheckReport`,
		func(rs m.AccountingReportSet) {
			//@api.multi
			/*def check_report(self):
			  res = super(AccountingReport, self).check_report()
			  data = {}
			  data['form'] = self.read(['account_report_id', 'date_from_cmp', 'date_to_cmp', 'journal_ids', 'filter_cmp', 'target_move'])[0]
			  for field in ['account_report_id']:
			      if isinstance(data['form'][field], tuple):
			          data['form'][field] = data['form'][field][0]
			  comparison_context = self._build_comparison_context(data)
			  res['data']['form']['comparison_context'] = comparison_context
			  return res

			*/
		})
	h.AccountingReport().Methods().PrintReport().DeclareMethod(
		`PrintReport`,
		func(rs m.AccountingReportSet, args struct {
			Data interface{}
		}) {
			/*def _print_report(self, data):
			  data['form'].update(self.read(['date_from_cmp', 'debit_credit', 'date_to_cmp', 'filter_cmp', 'account_report_id', 'enable_filter', 'label_filter', 'target_move'])[0])
			  return self.env['report'].get_action(self, 'account.report_financial', data=data)
			*/
		})

}
