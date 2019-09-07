package account

import (
	"testing"

	"github.com/hexya-addons/account/accounttypes"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

type TestTaxStruct struct {
	Super           TestAccountBaseUserStruct
	Env             models.Environment
	FixedTax        m.AccountTaxSet
	FixedTaxBis     m.AccountTaxSet
	PercentTax      m.AccountTaxSet
	DivisionTax     m.AccountTaxSet
	GroupTax        m.AccountTaxSet
	GroupTaxBis     m.AccountTaxSet
	GroupOfGroupTax m.AccountTaxSet
	BankJournal     m.AccountJournalSet
	BankAccount     m.AccountAccountSet
	ExpenseAccount  m.AccountAccountSet
}

func initTestTaxStruct(env models.Environment) TestTaxStruct {
	var out TestTaxStruct
	out.Env = env
	out.Super = initTestAccountBaseUserStruct(env)

	out.FixedTax = h.AccountTax().Create(env,
		h.AccountTax().NewData().
			SetName("Fixed Tax").
			SetAmountType("fixed").
			SetAmount(10).
			SetSequence(1))

	out.FixedTaxBis = h.AccountTax().Create(env,
		h.AccountTax().NewData().
			SetName("Fixed Tax Bis").
			SetAmountType("fixed").
			SetAmount(15).
			SetSequence(2))

	out.PercentTax = h.AccountTax().Create(env,
		h.AccountTax().NewData().
			SetName("Percent tax").
			SetAmountType("percent").
			SetAmount(10).
			SetSequence(3))

	out.DivisionTax = h.AccountTax().Create(env,
		h.AccountTax().NewData().
			SetName("Division tax").
			SetAmountType("division").
			SetAmount(10).
			SetSequence(4))

	out.GroupTax = h.AccountTax().Create(env,
		h.AccountTax().NewData().
			SetName("Group tax").
			SetAmountType("group").
			SetAmount(0).
			SetSequence(5).
			SetChildrenTaxes(out.FixedTax.Union(out.PercentTax)))

	out.GroupTaxBis = h.AccountTax().Create(env,
		h.AccountTax().NewData().
			SetName("Group tax bis").
			SetAmountType("group").
			SetAmount(0).
			SetSequence(6).
			SetChildrenTaxes(out.FixedTax.Union(out.PercentTax)))

	out.GroupOfGroupTax = h.AccountTax().Create(env,
		h.AccountTax().NewData().
			SetName("Group of Group Tax").
			SetAmountType("group").
			SetAmount(0).
			SetSequence(7).
			SetChildrenTaxes(out.GroupTax.Union(out.GroupTaxBis)))

	cond := q.AccountJournal().Type().Equals("bank").And().Company().Equals(out.Super.AccountManager.Company())
	out.BankJournal = h.AccountJournal().Search(env, cond).Records()[0]
	out.BankAccount = out.BankJournal.DefaultDebitAccount()
	cond2 := q.AccountAccount().UserTypeFilteredOn(q.AccountAccountType().Type().Equals("payable"))
	out.ExpenseAccount = h.AccountAccount().Search(env, cond2).Limit(1) // Should be done by onchange later

	return out
}

type computeAllOutStruct struct {
	base          float64
	totalExcluded float64
	totalIncluded float64
	taxes         []accounttypes.AppliedTaxData
}

func (tts TestTaxStruct) simpleComputeAll(tax m.AccountTaxSet, price float64) computeAllOutStruct {
	var out computeAllOutStruct
	out.base, out.totalExcluded, out.totalIncluded, out.taxes = tax.ComputeAll(
		price, h.Currency().NewSet(tts.Env), 1, h.ProductProduct().NewSet(tts.Env), h.Partner().NewSet(tts.Env))
	return out
}

func TestTaxGroupOfGroupTax(t *testing.T) {
	Convey("Test Tax Group Of Group Tax", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			tts := initTestTaxStruct(env)
			tts.FixedTax.SetIncludeBaseAmount(true)
			tts.GroupTax.SetIncludeBaseAmount(true)
			tts.GroupOfGroupTax.SetIncludeBaseAmount(true)
			res := tts.simpleComputeAll(tts.GroupOfGroupTax, 200)

			// After calculation of first group
			// base = 210
			// total_included = 231
			// Base of the first grouped is passed
			// Base after the second group (220) is dropped.
			// Base of the group of groups is passed out,
			// so we obtain base as after first group

			So(res.base, ShouldEqual, 210)
			So(res.totalExcluded, ShouldEqual, 200)
			So(res.totalIncluded, ShouldEqual, 263)
		}), ShouldBeNil)
	})
}

func TestTaxGroup(t *testing.T) {
	Convey("Test Tax Group", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			tts := initTestTaxStruct(env)
			res := tts.simpleComputeAll(tts.GroupTax, 200)
			So(res.totalExcluded, ShouldEqual, 200)
			So(res.totalIncluded, ShouldEqual, 230)
			So(len(res.taxes), ShouldEqual, 2)
			So(res.taxes[0].Amount, ShouldEqual, 10)
			So(res.taxes[1].Amount, ShouldEqual, 20)
		}), ShouldBeNil)
	})
}

func TestTaxPercentDivision(t *testing.T) {
	Convey("Test Tax Percent Division", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			tts := initTestTaxStruct(env)
			tts.DivisionTax.SetPriceInclude(true)
			tts.DivisionTax.SetIncludeBaseAmount(true)
			tts.PercentTax.SetPriceInclude(false)
			tts.PercentTax.SetIncludeBaseAmount(false)
			resDivision := tts.simpleComputeAll(tts.DivisionTax, 200)
			resPercent := tts.simpleComputeAll(tts.PercentTax, 200)
			So(resDivision.taxes[0].Amount, ShouldEqual, 20)
			So(resPercent.taxes[0].Amount, ShouldEqual, 20)
			tts.DivisionTax.SetPriceInclude(false)
			tts.DivisionTax.SetIncludeBaseAmount(false)
			tts.PercentTax.SetPriceInclude(true)
			tts.PercentTax.SetIncludeBaseAmount(true)
			resDivision = tts.simpleComputeAll(tts.DivisionTax, 200)
			resPercent = tts.simpleComputeAll(tts.PercentTax, 200)
			So(resDivision.taxes[0].Amount, ShouldEqual, 22.22)
			So(resPercent.taxes[0].Amount, ShouldEqual, 18.18)
		}), ShouldBeNil)
	})
}

func TestTaxSequenceNormalizedSet(t *testing.T) {
	Convey("Test Tax Sequence Normalized Set", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			tts := initTestTaxStruct(env)
			tts.DivisionTax.SetSequence(1)
			tts.FixedTax.SetSequence(2)
			tts.PercentTax.SetSequence(3)
			taxesSet := tts.GroupTax.Union(tts.DivisionTax)
			res := tts.simpleComputeAll(taxesSet, 200)
			So(res.taxes[0].Amount, ShouldEqual, 22.22)
			So(res.taxes[1].Amount, ShouldEqual, 10)
			So(res.taxes[2].Amount, ShouldEqual, 20)
		}), ShouldBeNil)
	})
}

func TestTaxIncludeBaseAmount(t *testing.T) {
	Convey("Test Tax Include Base Amount", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			tts := initTestTaxStruct(env)
			tts.FixedTax.SetIncludeBaseAmount(true)
			res := tts.simpleComputeAll(tts.GroupTax, 200)
			So(res.totalIncluded, ShouldEqual, 231)
		}), ShouldBeNil)
	})
}

func TestTaxCurrency(t *testing.T) {
	Convey("Test Tax Currency", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			tts := initTestTaxStruct(env)
			tts.DivisionTax.SetAmount(15)
			_, _, totalIncl, _ := tts.DivisionTax.ComputeAll(
				200, h.Currency().NewSet(tts.Env).GetRecord("base_VEF"), 1, h.ProductProduct().NewSet(tts.Env), h.Partner().NewSet(tts.Env))
			So(totalIncl, ShouldAlmostEqual, 235.2941)
		}), ShouldBeNil)
	})
}

// Test that creating a move.line with tax_ids generates the tax move lines and adjust line amount when a tax is price_include
func TestTaxMoveLinesCreation(t *testing.T) {
	Convey("Test Tax Move Lines Creation", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			tts := initTestTaxStruct(env)
			tts.FixedTax.SetPriceInclude(true)
			tts.FixedTax.SetIncludeBaseAmount(true)
			company := h.User().NewSet(env).CurrentUser().Company()

			move := h.AccountMove().NewSet(env).WithContext("apply_taxes", true).Create(
				h.AccountMove().NewData().
					SetDate(dates.Today().SetMonth(01).SetDay(01)).
					SetJournal(tts.BankJournal).
					SetName("Test move").
					SetCompany(company).
					CreateLines(h.AccountMoveLine().NewData().
						SetAccount(tts.BankAccount).
						SetDebit(235).
						SetCredit(0).
						SetName("Bank Fees")).
					CreateLines(h.AccountMoveLine().NewData().
						SetAccount(tts.ExpenseAccount).
						SetDebit(0).
						SetCredit(200).
						SetName("Bank Fees").
						SetTaxes(tts.GroupTax.Union(tts.FixedTaxBis))))

			amlFixedTax := move.Lines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.TaxLine().Equals(tts.FixedTax)
			})
			amlPercentTax := move.Lines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.TaxLine().Equals(tts.PercentTax)
			})
			amlFixedTaxBis := move.Lines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.TaxLine().Equals(tts.FixedTaxBis)
			})

			So(amlFixedTax.Len(), ShouldEqual, 1)
			So(amlFixedTax.Credit(), ShouldEqual, 10)
			So(amlPercentTax.Len(), ShouldEqual, 1)
			So(amlPercentTax.Credit(), ShouldEqual, 20)
			So(amlFixedTaxBis.Len(), ShouldEqual, 1)
			So(amlFixedTaxBis.Credit(), ShouldEqual, 15)

			amlWithTaxes := move.Lines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.Taxes().Equals(tts.GroupTax.Union(tts.FixedTaxBis))
			})
			So(amlWithTaxes.Len(), ShouldEqual, 1)
			So(amlWithTaxes.Credit(), ShouldEqual, 190)

		}), ShouldBeNil)
	})
}
