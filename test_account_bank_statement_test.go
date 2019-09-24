// Copyright 2019 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"testing"

	"github.com/hexya-addons/account/accounttypes"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

func TestBankStatement(t *testing.T) {
	Convey("Testing Bank Statement", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Bank statement test", func() {
				journal := h.AccountBankStatement().NewSet(env).
					WithContext("lang", "en_US").
					WithContext("tz", "").
					WithContext("active_model", "ir.ui.menu").
					WithContext("journal_type", "bank").
					WithContext("date", dates.Today()).DefaultJournal()
				So(journal.IsNotEmpty(), ShouldBeTrue)
				statement := h.AccountBankStatement().NewSet(env).WithContext("journal_type", "bank").
					Create(h.AccountBankStatement().NewData().
						SetBalanceEndReal(0).
						SetBalanceStart(0).
						SetDate(dates.Today()).
						SetCompany(h.Company().NewSet(env).GetRecord("base_main_company")))
				statementLine := h.AccountBankStatementLine().Create(env, h.AccountBankStatementLine().NewData().
					SetAmount(1000).
					SetDate(dates.Today()).
					SetPartner(h.Partner().NewSet(env).GetRecord("base_res_partner_4")).
					SetName("EXT001").
					SetStatement(statement))
				account := h.AccountAccount().Create(env, h.AccountAccount().NewData().
					SetName("toto").
					SetCode("bidule").
					SetUserType(h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_fixed_assets")))
				statementLine.ProcessReconciliation(h.AccountMoveLine().NewSet(env), []accounttypes.BankStatementAMLStruct{},
					[]accounttypes.BankStatementAMLStruct{
						{Credit: 1000, Debit: 0, Name: "toto", AccountID: account.ID()},
					})
				statement.SetBalanceEndReal(1000)
				statement.ButtonConfirmBank()
				So(statement.State(), ShouldEqual, "confirm")
			})
		}), ShouldBeNil)
	})
}

func TestAccountInvoiceState(t *testing.T) {
	Convey("Testing Account Invoice State", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Invoice test", func() {
				product := h.ProductProduct().NewSet(env).GetRecord("product_product_product_3")
				invoice := h.AccountInvoice().Create(env, h.AccountInvoice().NewData().
					SetCompany(h.Company().NewSet(env).GetRecord("base_main_company")).
					SetCurrency(h.Currency().NewSet(env).GetRecord("base_EUR")).
					SetPartner(h.Partner().NewSet(env).GetRecord("base_res_partner_12")).
					SetReferenceType("none").
					CreateInvoiceLines(h.AccountInvoiceLine().NewData().
						SetName("Computer SC234").
						SetPriceUnit(450).
						SetQuantity(1).
						SetProduct(product).
						SetAccount(h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(
							h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_revenue"))).Limit(1)).
						SetUom(h.ProductUom().NewSet(env).GetRecord("product_product_uom_unit"))))
				So(invoice.State(), ShouldEqual, "draft")
				wizard := h.AccountInvoiceConfirm().Create(env, h.AccountInvoiceConfirm().NewData())
				wizard.WithContext("lang", "en_US").
					WithContext("tz", "").
					WithContext("active_model", "AccountInvoice").
					WithContext("active_ids", invoice.Ids()).
					WithContext("type", "out_invoice").
					WithContext("active_id", invoice.ID()).InvoiceConfirm()
				So(invoice.State(), ShouldEqual, "open")
				moves := h.AccountMoveLine().Search(env, q.AccountMoveLine().Invoice().Equals(invoice))
				So(moves.Len(), ShouldBeGreaterThan, 0)
				moves.Records()[0].Journal().SetUpdatePosted(true)
				cancelWizard := h.AccountInvoiceCancel().Create(env, h.AccountInvoiceCancel().NewData())
				cancelWizard.WithContext("lang", "en_US").
					WithContext("tz", "").
					WithContext("active_model", "AccountInvoice").
					WithContext("active_ids", invoice.Ids()).
					WithContext("type", "out_invoice").
					WithContext("active_id", invoice.ID()).InvoiceCancel()
				So(invoice.State(), ShouldEqual, "cancel")
			})
		}), ShouldBeNil)
	})
}
