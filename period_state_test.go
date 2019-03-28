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

type TestPeriodStateStruct struct {
	Super        TestAccountBaseStruct
	User         m.UserSet
	LastDayMonth dates.Date
	SaleJournal  m.AccountJournalSet
	Account      m.AccountAccountSet
}

func initTestPeriodStateStruct(env models.Environment) TestPeriodStateStruct {
	var out TestPeriodStateStruct
	out.Super = initTestAccountBaseStruct(env)
	out.User = h.User().NewSet(env).CurrentUser()
	today := dates.Today()
	lastMonth := today.AddDate(0, -1, 0)
	isleap := 0
	if lastMonth.Year()%4 == 0 && (lastMonth.Year()%100 != 0 || lastMonth.Year()%400 == 0) {
		isleap = 1
	}
	mdays := []int{0, 31, 28 + isleap, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	lastDayMonth := lastMonth.SetDay(mdays[lastMonth.Month()])
	out.LastDayMonth = lastDayMonth
	journals := h.AccountJournal().Search(env, q.AccountJournal().Type().Equals("sale"))
	So(journals.IsNotEmpty(), ShouldBeTrue)
	out.SaleJournal = journals.Records()[0]
	accounts := h.AccountAccount().Search(env, q.AccountAccount().InternalType().Equals("receivable")).Records()[0]
	So(accounts.IsNotEmpty(), ShouldBeTrue)
	out.Account = accounts.Records()[0]
	return out
}

// Forbid creation of Journal Entries for a closed period
func TestPeriodState(t *testing.T) {
	Convey("Tests Period State", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestPeriodStateStruct(env)
			Convey("Test period state", func() {
				So(func() {
					lines := h.AccountMoveLine().NewSet(env).Create(
						h.AccountMoveLine().NewData().
							SetName("foo").
							SetDebit(10).
							SetAccount(self.Account)).
						Union(h.AccountMoveLine().Create(env,
							h.AccountMoveLine().NewData().
								SetName("bar").
								SetCredit(10).
								SetAccount(self.Account)))
					move := h.AccountMove().Create(env,
						h.AccountMove().NewData().
							SetName("/").
							SetJournal(self.SaleJournal).
							SetDate(self.LastDayMonth).
							SetLines(lines))
					move.Post()
				}, ShouldPanic)
			})
		}), ShouldBeNil)
	})
}
