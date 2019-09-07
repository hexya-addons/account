package account

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

func TestNameSearch(t *testing.T) {
	Convey("Test Name Search", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			acs := h.AccountAccountType().NewSet(env).SearchAll().Limit(1)

			atax := h.AccountAccount().Create(env,
				h.AccountAccount().NewData().
					SetName("Tax Received").
					SetCode("X121").
					SetUserType(acs).
					SetReconcile(true))

			apurchase := h.AccountAccount().Create(env,
				h.AccountAccount().NewData().
					SetName("Purchased Stocks").
					SetCode("X1101").
					SetUserType(acs).
					SetReconcile(true))

			asale := h.AccountAccount().Create(env,
				h.AccountAccount().NewData().
					SetName("Product Sales").
					SetCode("XX200").
					SetUserType(acs).
					SetReconcile(true))

			allIds := []int64{atax.ID(), apurchase.ID(), asale.ID()}

			ataxes := h.AccountAccount().NewSet(env).SearchByName("Tax", operator.IContains, q.AccountAccount().ID().In(allIds), 100)
			So(ataxes.Equals(atax), ShouldBeTrue) //name_search 'ilike Tax' should have returned Tax Received account only

			ataxes = h.AccountAccount().NewSet(env).SearchByName("Tax", operator.NotIContains, q.AccountAccount().ID().In(allIds), 100)
			So(ataxes.Equals(apurchase.Union(asale)), ShouldBeTrue) // name_search 'not ilike Tax' should have returned all but Tax Received account

			apurs := h.AccountAccount().NewSet(env).SearchByName("Purchased Stocks", operator.IContains, q.AccountAccount().ID().In(allIds), 100)
			So(apurs.Equals(apurchase), ShouldBeTrue) // name_search 'ilike Purchased Stocks' should have returned Purchased Stocks account only

			apurs = h.AccountAccount().NewSet(env).SearchByName("Purchased Stocks", operator.NotIContains, q.AccountAccount().ID().In(allIds), 100)
			So(apurs.Equals(atax.Union(asale)), ShouldBeTrue) // name_search 'not ilike Purchased Stocks' should have returned all but Purchased Stocks account

			asales := h.AccountAccount().NewSet(env).SearchByName("Product Sales", operator.IContains, q.AccountAccount().ID().In(allIds), 100)
			So(asales.Equals(asale), ShouldBeTrue) // name_search 'ilike Product Sales' should have returned Product Sales account only

			asales = h.AccountAccount().NewSet(env).SearchByName("Product Sales", operator.NotIContains, q.AccountAccount().ID().In(allIds), 100)
			So(asales.Equals(atax.Union(apurchase)), ShouldBeTrue) // name_search 'not ilike Product Sales' should have returned all but Product Sales account

			asales = h.AccountAccount().NewSet(env).SearchByName("XX200", operator.IContains, q.AccountAccount().ID().In(allIds), 100)
			So(asales.Equals(asale), ShouldBeTrue) // name_search 'ilike XX200' should have returned Product Sales account only

		}), ShouldBeNil)
	})
}

func TestPropertyUnsetSearch(t *testing.T) {
	Convey("Test Property Unset Search", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			aPartner := h.Partner().Create(env, h.Partner().NewData().SetName("test partner"))
			aPaymentTerm := h.AccountPaymentTerm().Create(env, h.AccountPaymentTerm().NewData().SetName("test payment term"))
			SearchCond := q.Partner().PropertyPaymentTerm().IsNull().And().ID().Equals(aPartner.ID())

			partners := h.Partner().Search(env, SearchCond)
			So(partners.IsNotEmpty(), ShouldBeTrue) // unset property field 'propety_payment_term' should have been found

			aPartner.Write(h.Partner().NewData().SetPropertyPaymentTerm(aPaymentTerm))

			partners = h.Partner().Search(env, SearchCond)
			So(partners.IsNotEmpty(), ShouldBeFalse) // unset property field 'propety_payment_term' should have NOT been found

		}), ShouldBeNil)
	})
}
