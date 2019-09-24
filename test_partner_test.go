// Copyright 2019 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	. "github.com/smartystreets/goconvey/convey"
)

func TestPartnerFieldsCompute(t *testing.T) {
	Convey("Checking partners computed fields", t, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			agrolait := h.Partner().NewSet(env).GetRecord("base_res_partner_2")
			Convey("BankAccountCount", func() {
				So(agrolait.BankAccountCount(), ShouldEqual, 0)
			})
			Convey("Credit / Debit", func() {
				So(agrolait.Credit(), ShouldEqual, 0)
				So(agrolait.Debit(), ShouldEqual, 0)
			})
			Convey("HasUnreconciledEntries", func() {
				So(agrolait.HasUnreconciledEntries(), ShouldBeFalse)
			})
			Convey("IssuedTotal", func() {
				So(agrolait.IssuedTotal(), ShouldEqual, 0)
			})
			Convey("JournalItemCount", func() {
				So(agrolait.JournalItemCount(), ShouldEqual, 0)
			})
			Convey("ContractsCount", func() {
				So(agrolait.ContractsCount(), ShouldEqual, 0)
			})
			Convey("Currency", func() {
				So(agrolait.Currency().Equals(h.Currency().NewSet(env).GetRecord("base_USD")), ShouldBeTrue)
			})
			Convey("TotalInvoiced", func() {
				So(agrolait.TotalInvoiced(), ShouldEqual, 0)
			})
		}), ShouldBeNil)
	})
}
