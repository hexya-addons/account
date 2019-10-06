package account

import (
	"testing"

	"github.com/hexya-addons/account/accounttypes"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

type TestBankStatementReconciliationStruct struct {
	Super           TestAccountBaseStruct
	Env             models.Environment
	PartnerAgrolait m.PartnerSet
}

func initTestBankStatementReconciliationStruct(env models.Environment) TestBankStatementReconciliationStruct {
	var out TestBankStatementReconciliationStruct
	out.Super = initTestAccountBaseStruct(env)
	out.PartnerAgrolait = h.Partner().NewSet(env).GetRecord("base_res_partner_2")
	out.Env = env
	return out
}

// Return the move line that gets to be reconciled (the one in the receivable account)
func (bs TestBankStatementReconciliationStruct) createInvoice(amount float64) m.AccountMoveLineSet {
	vals := h.AccountInvoice().NewData().
		SetPartner(bs.PartnerAgrolait).
		SetType("out_invoice").
		SetName("-").
		SetCurrency(h.User().NewSet(bs.Env).CurrentUser().Company().Currency())

	invoice := h.AccountInvoice().Create(bs.Env, vals)

	h.AccountInvoiceLine().Create(bs.Env,
		h.AccountInvoiceLine().NewData().
			SetQuantity(1).
			SetPriceUnit(amount).
			SetInvoice(invoice).
			SetName(".").
			SetAccount(h.AccountAccount().Search(bs.Env,
				q.AccountAccount().UserType().Equals(h.AccountAccountType().NewSet(bs.Env).GetRecord("account_data_account_type_revenue"))).Limit(1)))
	invoice.ActionInvoiceOpen()

	mvLine := h.AccountMoveLine().NewSet(bs.Env)
	for _, l := range invoice.Move().Lines().Records() {
		if l.Account().Equals(vals.Account()) {
			mvLine = l
		}
	}
	So(mvLine.IsNotEmpty(), ShouldBeTrue)
	return mvLine
}

func (bs TestBankStatementReconciliationStruct) createStatementLine(stLineAmount float64) m.AccountBankStatementLineSet {
	journal := h.AccountBankStatement().NewSet(bs.Env).WithContext("journal_type", "bank").DefaultJournal()
	bankStmt := h.AccountBankStatement().Create(bs.Env,
		h.AccountBankStatement().NewData().SetJournal(journal))
	bankStmtLine := h.AccountBankStatementLine().Create(bs.Env,
		h.AccountBankStatementLine().NewData().
			SetName("_").
			SetStatement(bankStmt).
			SetPartner(bs.PartnerAgrolait).
			SetAmount(stLineAmount))
	return bankStmtLine
}

func TestReconciliationProposition(t *testing.T) {
	Convey("Tests Empty", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestBankStatementReconciliationStruct(env)
			rcvMvLine := self.createInvoice(100)
			stLine := self.createStatementLine(100)

			// exact amount match
			recProp := stLine.GetReconciliationProposition([]int64{})
			So(recProp.Len(), ShouldEqual, 1)
			So(recProp.Records()[0].Equals(rcvMvLine), ShouldBeTrue)

		}), ShouldBeNil)
	})
}

func TestFullReconcile(t *testing.T) {
	Convey("Test Full Reconcile", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestBankStatementReconciliationStruct(env)
			rcvMvLine := self.createInvoice(100)
			stLine := self.createStatementLine(100)

			// reconcile
			counterpartAmlDicts := []accounttypes.BankStatementAMLStruct{{
				MoveLineID: rcvMvLine.ID(),
				Debit:      0,
				Credit:     100,
				Name:       rcvMvLine.Name(),
			}}
			stLine.ProcessReconciliation(h.AccountMoveLine().NewSet(env), counterpartAmlDicts, nil)

			// check everything went as expected
			recMove := stLine.JournalEntries().Records()[0]
			So(recMove.IsNotEmpty(), ShouldBeTrue)
			counterpartMvLine := h.AccountMoveLine().NewSet(env)
			for _, l := range recMove.Lines().Records() {
				if l.Account().UserType().Type() == "receivable" {
					counterpartMvLine = l
				}
			}
			So(counterpartMvLine.IsNotEmpty(), ShouldBeTrue)
			So(rcvMvLine.Reconciled(), ShouldBeTrue)
			So(counterpartMvLine.Reconciled(), ShouldBeTrue)
			So(counterpartMvLine.MatchedCredits().Equals(rcvMvLine.MatchedDebits()), ShouldBeTrue)
		}), ShouldBeNil)
	})
}

func TestReconcileWithWriteOff(t *testing.T) {
	Convey("Test Reconcile With Write Off", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestBankStatementReconciliationStruct(env)
			_ = self
		}), ShouldBeNil)
	})
}
