// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func init() {

	h.ProductCategory().AddFields(map[string]models.FieldDefinition{
		"PropertyAccountIncomeCateg": models.Many2OneField{
			String:        "Income Account",
			RelationModel: h.AccountAccount(), /*, CompanyDependent : true*/
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Help:          "This account will be used for invoices to value sales."},
		"PropertyAccountExpenseCateg": models.Many2OneField{
			String:        "Expense Account",
			RelationModel: h.AccountAccount(), /*, CompanyDependent : true*/
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Help:          "This account will be used for invoices to value expenses."},
	})

	h.ProductTemplate().AddFields(map[string]models.FieldDefinition{
		"Taxes": models.Many2ManyField{
			String:        "Customer Taxes",
			RelationModel: h.AccountTax(),
			JSON:          "taxes_id",
			Filter:        q.AccountTax().TypeTaxUse().Equals("sale")},
		"SupplierTaxes": models.Many2ManyField{
			String:        "Vendor Taxes",
			RelationModel: h.AccountTax(),
			JSON:          "supplier_taxes_id",
			Filter:        q.AccountTax().TypeTaxUse().Equals("purchase")},
		"PropertyAccountIncome": models.Many2OneField{
			String: "Income Account", RelationModel: h.AccountAccount(),
			/*, CompanyDependent : true*/
			Filter: q.AccountAccount().Deprecated().Equals(false),
			Help: `This account will be used for invoices instead of the default one
to value sales for the current product.`},
		"PropertyAccountExpense": models.Many2OneField{
			String: "Expense Account", RelationModel: h.AccountAccount(),
			/*, CompanyDependent : true*/
			Filter: q.AccountAccount().Deprecated().Equals(false),
			Help: `This account will be used for invoices instead of the default one
to value expenses for the current product.`},
	})

	h.ProductTemplate().Methods().Write().Extend("",
		func(rs m.ProductTemplateSet, data m.ProductTemplateData) bool {
			// should be a better way to do this with Hexya
			var uoms m.ProductTemplateSet
			check := rs.IsNotEmpty() && data.HasUomPo()
			cond := q.ProductTemplate().ID().In(rs.Ids())
			if check {
				uoms = h.ProductTemplate().Search(rs.Env(), cond)
			}
			res := rs.Super().Write(data)
			if check {
				if !h.ProductTemplate().Search(rs.Env(), cond).Equals(uoms) {
					products := h.ProductProduct().Search(rs.Env(), q.ProductProduct().ProductTmpl().In(rs))
					if h.AccountMoveLine().Search(rs.Env(), q.AccountMoveLine().Product().In(products)).Len() > 0 {
						panic(rs.T(`You can not change the unit of measure of a product that has been already used in an account journal item. If you need to change the unit of measure, you may deactivate this product.`))
					}
				}
			}
			return res
		})

	h.ProductTemplate().Methods().GetProductDirectAccounts().DeclareMethod(
		`GetProductDirectAccounts`,
		func(rs m.ProductTemplateSet) (m.AccountAccountSet, m.AccountAccountSet) {
			return h.AccountAccount().Coalesce(rs.PropertyAccountIncome(), rs.Category().PropertyAccountIncomeCateg()),
				h.AccountAccount().Coalesce(rs.PropertyAccountExpense(), rs.Category().PropertyAccountExpenseCateg())
		})

	h.ProductTemplate().Methods().GetAssetAccounts().DeclareMethod(
		`GetAssetAccounts`,
		func(rs m.ProductTemplateSet) (m.AccountAccountSet, m.AccountAccountSet) {
			return h.AccountAccount().NewSet(rs.Env()), h.AccountAccount().NewSet(rs.Env())
		})

	h.ProductTemplate().Methods().GetProductAccounts().DeclareMethod(
		`GetProductAccounts`,
		func(rs m.ProductTemplateSet, fiscalPos m.AccountFiscalPositionSet) (m.AccountAccountSet, m.AccountAccountSet) {
			income, expense := rs.GetProductDirectAccounts()
			m := map[string]m.AccountAccountSet{
				"income":  income,
				"expense": expense,
			}
			m = fiscalPos.MapAccounts(m)
			return m["income"], m["expense"]
		})

}
