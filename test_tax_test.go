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

func (self TestTaxStruct) simpleComputeAll(tax m.AccountTaxSet, price float64) computeAllOutStruct {
	var out computeAllOutStruct
	out.base, out.totalExcluded, out.totalIncluded, out.taxes = tax.ComputeAll(
		price, h.Currency().NewSet(self.Env), 1, h.ProductProduct().NewSet(self.Env), h.Partner().NewSet(self.Env))
	return out
}

func TestTaxGroupOfGroupTax(t *testing.T) {
	Convey("Test Tax Group Of Group Tax", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestTaxStruct(env)
			self.FixedTax.SetIncludeBaseAmount(true)
			self.GroupTax.SetIncludeBaseAmount(true)
			self.GroupOfGroupTax.SetIncludeBaseAmount(true)
			res := self.simpleComputeAll(self.GroupOfGroupTax, 200)
			/*
				# After calculation of first group
				# base = 210
				# total_included = 231
				# Base of the first grouped is passed
				# Base after the second group (220) is dropped.
				# Base of the group of groups is passed out,
				# so we obtain base as after first group
			*/
			So(res.base, ShouldEqual, 210)
			So(res.totalExcluded, ShouldEqual, 200)
			So(res.totalIncluded, ShouldEqual, 263)
		}), ShouldBeNil)
	})
}

func TestTaxGroup(t *testing.T) {
	Convey("Test Tax Group", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestTaxStruct(env)
			res := self.simpleComputeAll(self.GroupTax, 200)
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
			self := initTestTaxStruct(env)
			self.DivisionTax.SetPriceInclude(true)
			self.DivisionTax.SetIncludeBaseAmount(true)
			self.PercentTax.SetPriceInclude(false)
			self.PercentTax.SetIncludeBaseAmount(false)
			resDivision := self.simpleComputeAll(self.DivisionTax, 200)
			resPercent := self.simpleComputeAll(self.PercentTax, 200)
			So(resDivision.taxes[0].Amount, ShouldEqual, 20)
			So(resPercent.taxes[0].Amount, ShouldEqual, 20)
			self.DivisionTax.SetPriceInclude(false)
			self.DivisionTax.SetIncludeBaseAmount(false)
			self.PercentTax.SetPriceInclude(true)
			self.PercentTax.SetIncludeBaseAmount(true)
			resDivision = self.simpleComputeAll(self.DivisionTax, 200)
			resPercent = self.simpleComputeAll(self.PercentTax, 200)
			So(resDivision.taxes[0].Amount, ShouldEqual, 22.22)
			So(resPercent.taxes[0].Amount, ShouldEqual, 18.18)
		}), ShouldBeNil)
	})
}

func TestTaxSequenceNormalizedSet(t *testing.T) {
	Convey("Test Tax Sequence Normalized Set", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestTaxStruct(env)
			self.DivisionTax.SetSequence(1)
			self.FixedTax.SetSequence(2)
			self.PercentTax.SetSequence(3)
			taxesSet := self.GroupTax.Union(self.DivisionTax)
			res := self.simpleComputeAll(taxesSet, 200)
			So(res.taxes[0].Amount, ShouldEqual, 22.22)
			So(res.taxes[1].Amount, ShouldEqual, 10)
			So(res.taxes[2].Amount, ShouldEqual, 20)
		}), ShouldBeNil)
	})
}

func TestTaxIncludeBaseAmount(t *testing.T) {
	Convey("Test Tax Include Base Amount", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestTaxStruct(env)
			self.FixedTax.SetIncludeBaseAmount(true)
			res := self.simpleComputeAll(self.GroupTax, 200)
			So(res.totalIncluded, ShouldEqual, 231)
		}), ShouldBeNil)
	})
}

func TestTaxCurrency(t *testing.T) {
	Convey("Test Tax Currency", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestTaxStruct(env)
			self.DivisionTax.SetAmount(15)
			_, _, totalIncl, _ := self.DivisionTax.ComputeAll(
				200, h.Currency().NewSet(self.Env).GetRecord("base_VEF"), 1, h.ProductProduct().NewSet(self.Env), h.Partner().NewSet(self.Env))
			So(totalIncl, ShouldAlmostEqual, 235.2941)
		}), ShouldBeNil)
	})
}

// Test that creating a move.line with tax_ids generates the tax move lines and adjust line amount when a tax is price_include
func TestTaxMoveLinesCreation(t *testing.T) {
	Convey("Test Tax Move Lines Creation", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestTaxStruct(env)
			self.FixedTax.SetPriceInclude(true)
			self.FixedTax.SetIncludeBaseAmount(true)

			company := h.User().NewSet(env).CurrentUser().Company()

			line1 := h.AccountMoveLine().Create(env,
				h.AccountMoveLine().NewData().
					SetAccount(self.BankAccount).
					SetDebit(235).
					SetCredit(0).
					SetName("Bank Fees"))
			line2 := h.AccountMoveLine().Create(env,
				h.AccountMoveLine().NewData().
					SetAccount(self.ExpenseAccount).
					SetDebit(0).
					SetCredit(200).
					SetName("Bank Fees").
					SetTaxes(self.GroupTax.Union(self.FixedTaxBis)))

			move := h.AccountMove().NewSet(env).WithContext("apply_taxes", true).Create(
				h.AccountMove().NewData().
					SetDate(dates.Today().SetMonth(01).SetDay(01)).
					SetJournal(self.BankJournal).
					SetName("Test move").
					SetLines(line1.Union(line2)).
					SetCompany(company))

			amlFixedTax := move.Lines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.TaxLine().Equals(self.FixedTax)
			})
			amlPercentTax := move.Lines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.TaxLine().Equals(self.PercentTax)
			})
			amlFixedTaxBis := move.Lines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.TaxLine().Equals(self.FixedTaxBis)
			})

			So(amlFixedTax.Len(), ShouldEqual, 1)
			So(amlFixedTax.Credit(), ShouldEqual, 10)
			So(amlPercentTax.Len(), ShouldEqual, 1)
			So(amlPercentTax.Credit(), ShouldEqual, 20)
			So(amlFixedTaxBis.Len(), ShouldEqual, 1)
			So(amlFixedTaxBis.Credit(), ShouldEqual, 15)

			amlWithTaxes := move.Lines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.Taxes().Equals(self.GroupTax.Union(self.FixedTaxBis))
			})
			So(amlWithTaxes.Len(), ShouldEqual, 1)
			So(amlWithTaxes.Credit(), ShouldEqual, 190)

		}), ShouldBeNil)
	})
}
