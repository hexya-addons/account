package account

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

type TestProductIDChangeStruct struct {
	Super             TestAccountBaseStruct
	Env               models.Environment
	AccountReceivable m.AccountAccountSet
	AccountRevenue    m.AccountAccountSet
}

func initTestProductIDChangeStruct(env models.Environment) TestProductIDChangeStruct {
	var out TestProductIDChangeStruct
	out.Super = initTestAccountBaseStruct(env)
	out.Env = env
	out.AccountReceivable = h.AccountAccount().Search(env,
		q.AccountAccount().UserType().Equals(
			h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_receivable"))).Limit(1)
	out.AccountRevenue = h.AccountAccount().Search(env,
		q.AccountAccount().UserType().Equals(
			h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_revenue"))).Limit(1)
	return out
}

func TestProductIDChange(t *testing.T) {
	Convey("Test Product Id Change", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestProductIDChangeStruct(env)
			partner := h.Partner().Create(env, h.Partner().NewData().SetName("George"))

			taxIncludeSale := h.AccountTax().Create(env,
				h.AccountTax().NewData().
					SetName("Include Tax").
					SetTypeTaxUse("sale").
					SetAmount(21).
					SetPriceInclude(true))
			taxIncludePurchase := h.AccountTax().Create(env,
				h.AccountTax().NewData().
					SetName("Include Tax").
					SetTypeTaxUse("purchase").
					SetAmount(21).
					SetPriceInclude(true))
			taxExcludeSale := h.AccountTax().Create(env,
				h.AccountTax().NewData().
					SetName("Exclude Tax").
					SetTypeTaxUse("sale").
					SetAmount(0))
			taxExcludePurchase := h.AccountTax().Create(env,
				h.AccountTax().NewData().
					SetName("Exclude Tax").
					SetTypeTaxUse("purchase").
					SetAmount(0))

			productTmpl := h.ProductTemplate().Create(env,
				h.ProductTemplate().NewData().
					SetName("Voiture").
					SetListPrice(121).
					SetTaxes(taxIncludeSale).
					SetSupplierTaxes(taxIncludePurchase))
			product := h.ProductProduct().Create(env,
				h.ProductProduct().NewData().
					SetProductTmpl(productTmpl).
					SetStandardPrice(242))

			fp := h.AccountFiscalPosition().Create(env,
				h.AccountFiscalPosition().NewData().
					SetName("fiscal position").
					SetSequence(1))
			h.AccountFiscalPositionTax().Create(env,
				h.AccountFiscalPositionTax().NewData().
					SetPosition(fp).
					SetTaxSrc(taxIncludeSale).
					SetTaxDest(taxExcludeSale))
			h.AccountFiscalPositionTax().Create(env,
				h.AccountFiscalPositionTax().NewData().
					SetPosition(fp).
					SetTaxSrc(taxIncludePurchase).
					SetTaxDest(taxExcludePurchase))

			outInvoice := h.AccountInvoice().Create(env,
				h.AccountInvoice().NewData().
					SetPartner(partner).
					SetReferenceType("none").
					SetName("invoice to client").
					SetAccount(self.AccountReceivable).
					SetType("out_invoice").
					SetDateInvoice(dates.Today().SetMonth(06).SetDay(26)).
					SetFiscalPosition(fp))
			outLine := h.AccountInvoiceLine().Create(env,
				h.AccountInvoiceLine().NewData().
					SetProduct(product).
					SetQuantity(1).
					SetPriceUnit(121).
					SetInvoice(outInvoice).
					SetName("something out").
					SetAccount(self.AccountRevenue))
			inInvoice := h.AccountInvoice().Create(env,
				h.AccountInvoice().NewData().
					SetPartner(partner).
					SetReferenceType("none").
					SetName("invoice to supplier").
					SetAccount(self.AccountReceivable).
					SetType("in_invoice").
					SetDateInvoice(dates.Today().SetMonth(06).SetDay(26)).
					SetFiscalPosition(fp))
			inLine := h.AccountInvoiceLine().Create(env,
				h.AccountInvoiceLine().NewData().
					SetProduct(product).
					SetQuantity(1).
					SetPriceUnit(242).
					SetInvoice(inInvoice).
					SetName("something in").
					SetAccount(self.AccountRevenue))

			outLine.OnchangeProduct()
			So(outLine.PriceUnit(), ShouldEqual, 100)
			inLine.OnchangeProduct()
			So(inLine.PriceUnit(), ShouldEqual, 200)

		}), ShouldBeNil)
	})
}
