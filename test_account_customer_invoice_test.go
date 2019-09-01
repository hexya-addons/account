package account

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

func TestCustomerInvoice(t *testing.T) {
	Convey("Test Customer Invoice", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestAccountBaseUserStruct(env)
			// I will create bank detail with using manager access rights
			// because account manager can only create bank details.
			h.BankAccount().NewSet(env).Sudo(self.AccountManager.ID()).Create(
				h.BankAccount().NewData().
					SetCompany(self.MainCompany).
					SetPartner(self.MainPartner).
					SetName("123456789").
					SetBank(self.MainBank))

			// Test with that user which have rights to make Invoicing and payment and who is accountant.
			// Create a customer invoice
			paymentTerm := h.AccountPaymentTerm().NewSet(env).GetRecord("account_account_payment_term_advance")
			journalRec := h.AccountJournal().Search(env, q.AccountJournal().Type().Equals("sale")).Records()[0]
			partner3 := h.Partner().NewSet(env).GetRecord("base_res_partner_3")
			accountUserType := h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_receivable")
			accountUserTypeCurAssets := h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_current_assets")
			accountUserTypeRevenue := h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_revenue")
			ova := h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(accountUserTypeCurAssets)).Limit(1)

			//only adviser can create an account
			accountRec1 := h.AccountAccount().NewSet(env).Sudo(self.AccountManager.ID()).Create(
				h.AccountAccount().NewData().
					SetCode("cust_acc").
					SetName("Customer Account").
					SetUserType(accountUserType).
					SetReconcile(true))

			accountInvoiceCustomer0 := h.AccountInvoice().NewSet(env).Sudo(self.AccountUser.ID()).Create(
				h.AccountInvoice().NewData().
					SetName("Test Customer Invoice").
					SetReferenceType("none").
					SetPaymentTerm(paymentTerm).
					SetJournal(journalRec).
					SetPartner(partner3).
					SetAccount(accountRec1).
					CreateInvoiceLines(h.AccountInvoiceLine().NewData().
						SetProduct(h.ProductProduct().NewSet(env).GetRecord("product_product_product_5")).
						SetQuantity(10).
						SetAccount(h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(accountUserTypeRevenue)).Limit(1)).
						SetName("product test 5").
						SetPriceUnit(100)))

			// I manually assign tax on invoice
			tax := h.AccountInvoiceTax().Create(env,
				h.AccountInvoiceTax().NewData().
					SetName("Test tax for Customer Invoice").
					SetManual(true).
					SetAmount(9050).
					SetAccount(ova).
					SetInvoice(accountInvoiceCustomer0))
			So(tax.IsNotEmpty(), ShouldBeTrue)

			totalBeforeConfirm := partner3.TotalInvoiced()

			// I check that Initially customer invoice is in the "Draft" state
			So(accountInvoiceCustomer0.State(), ShouldEqual, "draft")

			// I change the state of invoice to "Proforma2" by clicking PRO-FORMA button
			accountInvoiceCustomer0.ActionInvoiceProforma2()

			// I check that the invoice state is now "Proforma2"
			So(accountInvoiceCustomer0.State(), ShouldEqual, "proforma2")

			// I check that there is no move attached to the invoice
			So(accountInvoiceCustomer0.Move().IsEmpty(), ShouldBeTrue)

			// I validate invoice by creating on
			accountInvoiceCustomer0.ActionInvoiceOpen()

			// I check that the invoice is valid
			So(accountInvoiceCustomer0.Move().IsNotEmpty(), ShouldBeTrue)
			So(accountInvoiceCustomer0.Type(), ShouldEqual, "out_invoice")
			So(accountInvoiceCustomer0.AmountTotal(), ShouldEqual, 10050)
			So(accountInvoiceCustomer0.AmountTotalSigned(), ShouldEqual, 10050)
			So(accountInvoiceCustomer0.AmountTotalCompanySigned(), ShouldEqual, 10050)

			// I check that the invoice state is "Open"
			So(accountInvoiceCustomer0.State(), ShouldEqual, "open")
			So(accountInvoiceCustomer0.Residual(), ShouldEqual, 10050)

			// I totally pay the Invoice
			accountInvoiceCustomer0.PayAndReconcile(
				h.AccountJournal().Search(env, q.AccountJournal().Type().Equals("bank")).Limit(1),
				10050, dates.Date{}, h.AccountAccount().NewSet(env))

			// I verify that invoice is now in Paid state
			So(accountInvoiceCustomer0.State(), ShouldEqual, "paid")

			totalAfterConfirm := partner3.TotalInvoiced()
			So(totalAfterConfirm-totalBeforeConfirm, ShouldEqual, accountInvoiceCustomer0.AmountUntaxedSigned())

			// I refund the invoice Using Refund Button
			accountInvoiceRefund := h.AccountInvoiceRefund().Create(env,
				h.AccountInvoiceRefund().NewData().
					SetDescription("Refund to China Export").
					SetDate(dates.Today()).
					SetFilterRefund("refund"))

			// I clicked on refund button.
			accountInvoiceRefund.InvoiceRefund()
		}), ShouldBeNil)
	})
}

func TestCustomerInvoiceTax(t *testing.T) {
	Convey("Test Customer Invoice Tax", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			h.User().NewSet(env).CurrentUser().Company().SetTaxCalculationRoundingMethod("round_globally")

			paymentTerm := h.AccountPaymentTerm().NewSet(env).GetRecord("account_account_payment_term_advance")
			journalRec := h.AccountJournal().Search(env, q.AccountJournal().Type().Equals("sale")).Limit(1)
			partner := h.Partner().NewSet(env).GetRecord("base_res_partner_3")
			accountTypeRevenue := h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_revenue")
			account := h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(accountTypeRevenue)).Limit(1)

			tax := h.AccountTax().Create(env,
				h.AccountTax().NewData().
					SetName("Tax 15.0").
					SetAmount(15.0).
					SetAmountType("percent").
					SetTypeTaxUse("sale"))

			invoiceLineBaseData := h.AccountInvoiceLine().NewData().
				SetAccount(account).
				SetDiscount(10).
				SetPriceUnit(2.77).
				SetInvoiceLineTaxes(tax)

			invoiceData := h.AccountInvoice().NewData().
				SetName("Test Customer Invoice").
				SetReferenceType("none").
				SetPaymentTerm(paymentTerm).
				SetJournal(journalRec).
				SetPartner(partner).
				CreateInvoiceLines(invoiceLineBaseData.Copy().
					SetProduct(h.ProductProduct().NewSet(env).GetRecord("product_product_product_1")).
					SetName("product test 1").
					SetQuantity(40).
					SetPriceUnit(2.27)).
				CreateInvoiceLines(invoiceLineBaseData.Copy().
					SetProduct(h.ProductProduct().NewSet(env).GetRecord("product_product_product_2")).
					SetName("product test 2").
					SetQuantity(21)).
				CreateInvoiceLines(invoiceLineBaseData.Copy().
					SetProduct(h.ProductProduct().NewSet(env).GetRecord("product_product_product_3")).
					SetName("product test 3").
					SetQuantity(21))

			invoice := h.AccountInvoice().Create(env, invoiceData)

			var total float64
			for _, x := range invoice.TaxLines().Records() {
				total += x.Base()
			}
			So(invoice.AmountUntaxed(), ShouldAlmostEqual, total)
		}), ShouldBeNil)
	})
}

func TestCustomerInvoiceTaxRefund(t *testing.T) {
	Convey("Test Customer Invoice Tax Refund", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			company := h.User().NewSet(env).CurrentUser().Company()
			accTypeCurAssets := h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_current_assets")
			accTypeRevenue := h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_revenue")

			taxAccount := h.AccountAccount().Create(env,
				h.AccountAccount().NewData().
					SetName("TAX").
					SetCode("TAX").
					SetUserType(accTypeCurAssets).
					SetCompany(company))
			taxAccountRefund := h.AccountAccount().Create(env,
				h.AccountAccount().NewData().
					SetName("TAX_REFUND").
					SetCode("TAX_R").
					SetUserType(accTypeCurAssets).
					SetCompany(company))

			journal := h.AccountJournal().Search(env, q.AccountJournal().Type().Equals("sale")).Limit(1)
			partner := h.Partner().NewSet(env).GetRecord("base_res_partner_3")
			account := h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(accTypeRevenue)).Limit(1)

			tax := h.AccountTax().Create(env,
				h.AccountTax().NewData().
					SetName("Tax 15.0").
					SetAmountType("percent").
					SetAmount(15).
					SetTypeTaxUse("sale").
					SetAccount(taxAccount).
					SetRefundAccount(taxAccountRefund))

			invoice := h.AccountInvoice().Create(env,
				h.AccountInvoice().NewData().
					SetName("Test Customer Invoice").
					SetReferenceType("none").
					SetJournal(journal).
					SetPartner(partner).
					CreateInvoiceLines(h.AccountInvoiceLine().NewData().
						SetProduct(h.ProductProduct().NewSet(env).GetRecord("product_product_product_1")).
						SetQuantity(40).
						SetAccount(account).
						SetName("product test 1").
						SetDiscount(10).
						SetPriceUnit(2.27).
						SetInvoiceLineTaxes(tax)))

			invoice.ActionInvoiceOpen()
			refund := invoice.Refund(dates.Date{}, dates.Date{}, "", h.AccountJournal().NewSet(env))

			taxAccounts := h.AccountAccount().NewSet(env)
			for _, taxline := range invoice.TaxLines().Records() {
				taxAccounts = taxAccounts.Union(taxline.Account())
			}
			taxRefundAccounts := h.AccountAccount().NewSet(env)
			for _, taxline := range refund.TaxLines().Records() {
				taxRefundAccounts = taxRefundAccounts.Union(taxline.Account())
			}

			So(taxAccounts.Equals(taxAccount), ShouldBeTrue)
			So(taxRefundAccounts.Equals(taxAccountRefund), ShouldBeTrue)

		}), ShouldBeNil)
	})
}
