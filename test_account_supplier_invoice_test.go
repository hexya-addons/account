package account

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

func TestSupplierInvoice(t *testing.T) {
	Convey("Test Supplier Invoice", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			accountTypeReceivable := h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_receivable")
			accountTypeExpenses := h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_expenses")
			partner2 := h.Partner().NewSet(env).GetRecord("base_res_partner_2")
			Product4 := h.ProductProduct().NewSet(env).GetRecord("product_product_product_4")

			tax := h.AccountTax().Create(env,
				h.AccountTax().NewData().
					SetName("Tax 10.0").
					SetAmount(10).
					SetAmountType("fixed"))
			analyticAccount := h.AccountAnalyticAccount().Create(env,
				h.AccountAnalyticAccount().NewData().
					SetName("Test Account"))

			// Should be changed by automatic on_change later
			invoiceAccount := h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(accountTypeReceivable)).Limit(1)
			invoiceLineAccount := h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(accountTypeExpenses)).Limit(1)

			invoice := h.AccountInvoice().Create(env,
				h.AccountInvoice().NewData().
					SetPartner(partner2).
					SetAccount(invoiceAccount).
					SetType("in_invoice"))

			h.AccountInvoiceLine().Create(env,
				h.AccountInvoiceLine().NewData().
					SetProduct(Product4).
					SetQuantity(1).
					SetPriceUnit(100).
					SetInvoice(invoice).
					SetName("product that costs 100").
					SetAccount(invoiceLineAccount).
					SetInvoiceLineTaxes(tax).
					SetAccountAnalytic(analyticAccount))

			// check that Initially supplier bill state is "Draft"
			So(invoice.State(), ShouldEqual, "draft")

			// change the state of invoice to open by clicking Validate button
			invoice.ActionInvoiceOpen()

			// I cancel the account move which is in posted state and verifies that it gives warning message
			So(func() { invoice.Move().ButtonCancel() }, ShouldPanic)
		}), ShouldBeNil)
	})
}

func TestSupplierInvoice2(t *testing.T) {
	Convey("Test Supplier Invoice 2", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			taxFixed := h.AccountTax().Create(env,
				h.AccountTax().NewData().
					SetSequence(10).
					SetName("Tax 10.0 (Fixed)").
					SetAmount(10).
					SetAmountType("fixed").
					SetIncludeBaseAmount(true))
			taxPercentIncludedBaseIncl := h.AccountTax().Create(env,
				h.AccountTax().NewData().
					SetSequence(20).
					SetName("Tax 50.0% (Percentage of Price Tax Included)").
					SetAmount(50).
					SetAmountType("division").
					SetIncludeBaseAmount(true))
			taxPercentage := h.AccountTax().Create(env,
				h.AccountTax().NewData().
					SetSequence(30).
					SetName("Tax 20.0% (Percentage of Price)").
					SetAmount(20).
					SetAmountType("percent").
					SetIncludeBaseAmount(false))
			analyticAccount := h.AccountAnalyticAccount().Create(env,
				h.AccountAnalyticAccount().NewData().
					SetName("test account"))

			accountTypeReceivable := h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_receivable")
			AccountTypeExpenses := h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_expenses")
			partner2 := h.Partner().NewSet(env).GetRecord("base_res_partner_2")
			Product4 := h.ProductProduct().NewSet(env).GetRecord("product_product_product_4")

			// Should be changed by automatic on_change later
			invoiceAccount := h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(accountTypeReceivable)).Limit(1)
			invoiceLineAccount := h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(AccountTypeExpenses)).Limit(1)

			invoice := h.AccountInvoice().Create(env,
				h.AccountInvoice().NewData().
					SetPartner(partner2).
					SetAccount(invoiceAccount).
					SetType("in_invoice"))

			h.AccountInvoiceLine().Create(env,
				h.AccountInvoiceLine().NewData().
					SetProduct(Product4).
					SetQuantity(5).
					SetPriceUnit(100).
					SetInvoice(invoice).
					SetName("product that costs 100").
					SetAccount(invoiceLineAccount).
					SetInvoiceLineTaxes(taxFixed.Union(taxPercentIncludedBaseIncl).Union(taxPercentage)).
					SetAccountAnalytic(analyticAccount))
			invoice.ComputeTaxes()

			// check that Initially supplier bill state is "Draft"
			So(invoice.State(), ShouldEqual, "draft")

			// change the state of invoice to open by clicking Validate button
			invoice.ActionInvoiceOpen()

			// Check if amount and corresponded base is correct for all tax scenarios given on a computational base
			// Keep in mind that tax amount can be changed by the user at any time before validating (based on the invoice and tax laws applicable)
			invoiceTax := invoice.TaxLines().Sorted(func(set m.AccountInvoiceTaxSet, set2 m.AccountInvoiceTaxSet) bool {
				return set.Sequence() <= set2.Sequence()
			})

			var amounts []float64
			var bases []float64

			for _, tax := range invoiceTax.Records() {
				if val := tax.Amount(); val != 0.0 {
					amounts = append(amounts, val)
				}
				if val := tax.Base(); val != 0.0 {
					bases = append(bases, val)
				}
			}
			So(amounts, ShouldHaveLength, 3)
			So(amounts[0], ShouldEqual, 50)
			So(amounts[1], ShouldEqual, 550)
			So(amounts[2], ShouldEqual, 220)
			So(bases, ShouldHaveLength, 3)
			So(bases[0], ShouldEqual, 500)
			So(bases[1], ShouldEqual, 550)
			So(bases[2], ShouldEqual, 1100)

			// I cancel the account move which is in posted state and verifies that it gives warning message
			So(func() { invoice.Move().ButtonCancel() }, ShouldPanic)
		}), ShouldBeNil)
	})
}
