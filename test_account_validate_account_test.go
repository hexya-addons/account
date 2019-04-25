package account

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

func TestAccountValidateAccount(t *testing.T) {
	Convey("Test AccountValidateAccount", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			Convey("Test account validate account", func() {
				accountCash := h.AccountAccount().Search(env, q.AccountAccount().UserTypeFilteredOn(q.AccountAccountType().Type().Equals("liquidity"))).Limit(1)
				journal := h.AccountJournal().Search(env, q.AccountJournal().Type().Equals("bank")).Limit(1)
				company := h.User().NewSet(env).CurrentUser().Company()

				// create move
				move := h.AccountMove().Create(env, h.AccountMove().NewData().
					SetName("/").
					SetRef("2011010").
					SetJournal(journal).
					SetState("draft").
					SetCompany(company))

				//create move lines
				data := h.AccountMoveLine().NewData().
					SetAccount(accountCash).
					SetName("Basic Computer").
					SetMove(move)
				h.AccountMoveLine().Create(env, data)
				h.AccountMoveLine().Create(env, data)

				// check that Initially account move state is "Draft"
				So(move.State(), ShouldEqual, "draft")

				// validate this account move by using the 'Post Journal Entries' wizard
				validateAccountMove := h.ValidateAccountMove().NewSet(env).WithContext("active_ids", []int64{move.ID()}).Create(h.ValidateAccountMove().NewData())

				// click on validate Button
				validateAccountMove.WithContext("active_ids", []int64{move.ID()}).ValidateMove()

				// check that the move state is now "Posted"
				So(move.State(), ShouldEqual, "posted")
			})
		}), ShouldBeNil)
	})
}
