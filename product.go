// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"math"

	"github.com/hexya-addons/account/accounttypes"
	"github.com/hexya-addons/base"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func init() {

	h.ProductCategory().AddFields(map[string]models.FieldDefinition{
		"PropertyAccountIncomeCateg": models.Many2OneField{
			String:        "Income Account",
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Help:          "This account will be used for invoices to value sales.",
			Contexts:      base.CompanyDependent,
			Default: func(env models.Environment) interface{} {
				return h.AccountAccount().NewSet(env).GetDefaultAccountFromChart("PropertyAccountIncomeCateg")
			}},
		"PropertyAccountExpenseCateg": models.Many2OneField{
			String:        "Expense Account",
			RelationModel: h.AccountAccount(), /*, CompanyDependent : true*/
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Help:          "This account will be used for invoices to value expenses.",
			Contexts:      base.CompanyDependent,
			Default: func(env models.Environment) interface{} {
				return h.AccountAccount().NewSet(env).GetDefaultAccountFromChart("PropertyAccountExpenseCateg")
			}},
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
			Filter: q.AccountAccount().Deprecated().Equals(false),
			Help: `This account will be used for invoices instead of the default one
to value sales for the current product.`,
			Contexts: base.CompanyDependent,
			Default: func(env models.Environment) interface{} {
				return h.AccountAccount().NewSet(env).GetDefaultAccountFromChart("PropertyAccountIncome")
			}},
		"PropertyAccountExpense": models.Many2OneField{
			String: "Expense Account", RelationModel: h.AccountAccount(),
			Filter: q.AccountAccount().Deprecated().Equals(false),
			Help: `This account will be used for invoices instead of the default one
to value expenses for the current product.`,
			Contexts: base.CompanyDependent,
			Default: func(env models.Environment) interface{} {
				return h.AccountAccount().NewSet(env).GetDefaultAccountFromChart("PropertyAccountExpense")
			}},
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

	h.ProductProduct().Methods().ConvertPreparedAnglosaxonLine().DeclareMethod(
		`ConvertPreparedAnglosaxonLine transforms the given accounttype.InvoiceLineAMLStruct 
		into a m.AccountInvoiceLineData valid for move creation.`,
		func(rs m.ProductProductSet, line accounttypes.InvoiceLineAMLStruct, partner m.PartnerSet) m.AccountMoveLineData {
			res := h.AccountMoveLine().NewData()
			var credit, debit, aCurrency float64
			if line.Price > 0 {
				debit = line.Price
				aCurrency = math.Abs(line.AmountCurrency)
			} else {
				credit = -line.Price
				aCurrency = -math.Abs(line.AmountCurrency)
			}
			qty := line.Quantity
			if qty == 0 {
				qty = 1
			}
			res.SetDateMaturity(line.DateMaturity).
				SetPartner(partner).
				SetName(line.Name).
				SetCredit(credit).
				SetDebit(debit).
				SetAccount(h.AccountAccount().BrowseOne(rs.Env(), line.AccountID)).
				SetAnalyticLines(h.AccountAnalyticLine().Browse(rs.Env(), line.AnalyticLinesIDs)).
				SetAmountCurrency(aCurrency).
				SetCurrency(h.Currency().BrowseOne(rs.Env(), line.CurrencyID)).
				SetQuantity(qty).
				SetProduct(h.ProductProduct().BrowseOne(rs.Env(), line.ProductID)).
				SetProductUom(h.ProductUom().BrowseOne(rs.Env(), line.UomID)).
				SetAnalyticAccount(h.AccountAnalyticAccount().BrowseOne(rs.Env(), line.AccountAnalyticID)).
				SetInvoice(h.AccountInvoice().BrowseOne(rs.Env(), line.InvoiceID)).
				SetTaxes(h.AccountTax().Browse(rs.Env(), line.TaxIDs)).
				SetTaxLine(h.AccountTax().BrowseOne(rs.Env(), line.TaxLineID)).
				SetAnalyticTags(h.AccountAnalyticTag().Browse(rs.Env(), line.AnalyticTagsIDs))
			return res
		})

}
