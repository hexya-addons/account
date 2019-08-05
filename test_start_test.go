package account

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/tests"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

func TestMain(m *testing.M) {

	tests.RunTests(m, "account", func() {
		err := models.ExecuteInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			chart := h.AccountChartTemplate().NewSet(env).GetRecord("l10n_generic_coa_configurable_chart_template")
			chart.TryLoadingForCurrentCompany()
		})
		if err != nil {
			panic(err)
		}
	})
}

type TestAccountBaseStruct struct {
}

func initTestAccountBaseStruct(env models.Environment) TestAccountBaseStruct {
	var out TestAccountBaseStruct
	err := models.ExecuteInNewEnvironment(1, func(env models.Environment) {
		gr := h.Group().Search(env, q.Group().GroupID().Equals("account_group_account_user"))
		demoUser := h.User().NewSet(env).GetRecord("base_user_demo")
		demoUser.SetGroups(demoUser.Groups().Union(gr))
	})
	if err != nil {
		panic(err)
	}
	mainCpny := h.Company().NewSet(env).GetRecord("base_main_company")
	domain := q.AccountAccount().Company().Equals(mainCpny)
	if h.AccountAccount().Search(env, domain).IsEmpty() {
		panic("Skipping test: No Chart of account found")
	}
	return out
}

type TestAccountBaseUserStruct struct {
	Super          TestAccountBaseStruct
	MainCompany    m.CompanySet
	MainPartner    m.PartnerSet
	MainBank       m.BankSet
	CurrencyEuro   m.CurrencySet
	AccountUser    m.UserSet
	AccountManager m.UserSet
}

func initTestAccountBaseUserStruct(env models.Environment) TestAccountBaseUserStruct {
	var out TestAccountBaseUserStruct
	out.Super = initTestAccountBaseStruct(env)
	out.MainCompany = h.Company().NewSet(env).GetRecord("base_main_company")
	out.MainPartner = h.Partner().NewSet(env).GetRecord("base_main_partner")
	out.MainBank = h.Bank().NewSet(env).GetRecord("base_res_bank_1")
	out.CurrencyEuro = h.Currency().NewSet(env).GetRecord("base_EUR")
	groupsUser := h.Group().Search(env, q.Group().GroupID().Equals("account_group_account_user")).Union(
		h.Group().Search(env, q.Group().GroupID().Equals("base_group_partner_manager"))).Union(
		h.Group().Search(env, q.Group().GroupID().Equals("analytic_group_analytic_accounting")))
	groupsManager := h.Group().Search(env, q.Group().GroupID().Equals("account_group_account_manager")).Union(
		h.Group().Search(env, q.Group().GroupID().Equals("base_group_partner_manager")))
	out.AccountUser = h.User().NewSet(env).WithContext("no_reset_password", true).Create(
		h.User().NewData().
			SetName("Accountant").
			SetCompany(out.MainCompany).
			SetLogin("acc").
			SetEmail("accountuser@yourcompany.com").
			SetGroups(groupsUser))
	out.AccountManager = h.User().NewSet(env).WithContext("no_reset_password", true).Create(
		h.User().NewData().
			SetName("Adiser").
			SetCompany(out.MainCompany).
			SetLogin("fm").
			SetEmail("accountmanager@yourcompany.com").
			SetGroups(groupsManager))
	out.AccountUser.SyncMemberships()
	out.AccountManager.SyncMemberships()
	return out
}

func TestEmptyTest(t *testing.T) {
	Convey("Tests Empty", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			partner := h.AccountMove().NewData().Partner()
			partner.ID()
		}), ShouldBeNil)
	})
}
