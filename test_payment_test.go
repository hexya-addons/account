package account

import (
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

type TestPaymentStruct struct {
	Super                  TestAccountBaseStruct
	Env                    models.Environment
	PartnerAgrolait        m.PartnerSet
	PartnerAxelor          m.PartnerSet
	CurrencyChf            m.CurrencySet
	CurrencyUsd            m.CurrencySet
	CurrencyEur            m.CurrencySet
	Product                m.ProductProductSet
	PaymentMethodManualIn  m.AccountPaymentMethodSet
	PaymentMethodManualOut m.AccountPaymentMethodSet
	AccountReceivable      m.AccountAccountSet
	AccountPayable         m.AccountAccountSet
	AccountRevenue         m.AccountAccountSet
	BankJournalEuro        m.AccountJournalSet
	AccountEur             m.AccountAccountSet
	BankJournalUsd         m.AccountJournalSet
	AccountUsd             m.AccountAccountSet
	TransferAccount        m.AccountAccountSet
	DiffIncomeAccount      m.AccountAccountSet
	DiffExpenseAccount     m.AccountAccountSet
}

type TestAMLStruct struct {
	Account        m.AccountAccountSet
	Debit          float64
	Credit         float64
	AmountCurrency float64
	Currency       m.CurrencySet
	CurrencyDiff   float64
}

func initTestPaymentStruct(env models.Environment) TestPaymentStruct {
	var out TestPaymentStruct
	out.Super = initTestAccountBaseStruct(env)
	out.Env = env

	out.PartnerAgrolait = h.Partner().NewSet(env).GetRecord("base_res_partner_2")
	out.PartnerAxelor = h.Partner().NewSet(env).GetRecord("base_res_partner_2")
	out.CurrencyChf = h.Currency().NewSet(env).GetRecord("base_CHF")
	out.CurrencyUsd = h.Currency().NewSet(env).GetRecord("base_USD")
	out.CurrencyEur = h.Currency().NewSet(env).GetRecord("base_EUR")

	company := h.Company().NewSet(env).GetRecord("base_main_company")
	company.SetCurrency(out.CurrencyEur)
	out.Product = h.ProductProduct().NewSet(env).GetRecord("product_product_product_4")
	out.PaymentMethodManualIn = h.AccountPaymentMethod().NewSet(env).GetRecord("account_account_payment_method_manual_in")
	out.PaymentMethodManualOut = h.AccountPaymentMethod().NewSet(env).GetRecord("account_account_payment_method_manual_out")

	out.AccountReceivable = h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_receivable"))).Limit(1)
	out.AccountPayable = h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_payable"))).Limit(1)
	out.AccountRevenue = h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_revenue"))).Limit(1)

	out.BankJournalEuro = h.AccountJournal().Create(env,
		h.AccountJournal().NewData().
			SetName("Bank").
			SetType("bank").
			SetCode("BNK67"))
	out.BankJournalUsd = h.AccountJournal().Create(env,
		h.AccountJournal().NewData().
			SetName("Bank US").
			SetType("bank").
			SetCode("BNK68").
			SetCurrency(out.CurrencyUsd))

	out.AccountEur = out.BankJournalEuro.DefaultDebitAccount()
	out.AccountUsd = out.BankJournalUsd.DefaultDebitAccount()

	usrCmpny := h.User().NewSet(env).CurrentUser().Company()

	out.TransferAccount = usrCmpny.TransferAccount()
	out.DiffIncomeAccount = usrCmpny.IncomeCurrencyExchangeAccount()
	out.DiffExpenseAccount = usrCmpny.ExpenseCurrencyExchangeAccount()

	return out
}

// CreateInvoice Returns an open invoice
func (tps TestPaymentStruct) CreateInvoice(amount float64, typ string, currency m.CurrencySet) m.AccountInvoiceSet {
	if amount == 0.0 {
		amount = 100.0
	}
	if typ == "" {
		typ = "out_invoice"
	}
	invoiceName := "invoice to supplier"
	if typ == "out_invoice" {
		invoiceName = "invoice to client"
	}

	invoice := h.AccountInvoice().Create(tps.Env, h.AccountInvoice().NewData().
		SetPartner(tps.PartnerAgrolait).
		SetReferenceType("none").
		SetCurrency(currency).
		SetName(invoiceName).
		SetAccount(tps.AccountReceivable).
		SetType(typ).
		SetDateInvoice(dates.Today().SetMonth(6).SetDay(26)))

	h.AccountInvoiceLine().Create(tps.Env, h.AccountInvoiceLine().NewData().
		SetProduct(tps.Product).
		SetQuantity(1).
		SetPriceUnit(amount).
		SetInvoice(invoice).
		SetName("something").
		SetAccount(tps.AccountRevenue))

	invoice.ActionInvoiceOpen()
	return invoice

}

// Reconcile reconcile a journal entry corresponding to a payment with its bank statement line
func (tps TestPaymentStruct) Reconcile(liquidityAml m.AccountMoveLineSet, amount, amountCurrency float64, currency m.CurrencySet) m.AccountBankStatementSet {
	date := dates.Today().SetMonth(7).SetDay(15)
	bankStmt := h.AccountBankStatement().Create(tps.Env, h.AccountBankStatement().NewData().
		SetJournal(liquidityAml.Journal()).
		SetDate(date))
	bankStmtLine := h.AccountBankStatementLine().Create(tps.Env, h.AccountBankStatementLine().NewData().
		SetName("payment").
		SetStatement(bankStmt).
		SetPartner(tps.PartnerAgrolait).
		SetAmount(amount).
		SetAmountCurrency(amountCurrency).
		SetCurrency(currency).
		SetDate(date))
	bankStmtLine.ProcessReconciliation(liquidityAml, nil, nil)
	return bankStmt
}

func (tps TestPaymentStruct) CheckJournalItems(amlRecs m.AccountMoveLineSet, amlDatas []TestAMLStruct) {
	compareRecData := func(amlr m.AccountMoveLineSet, amld TestAMLStruct) bool {
		if amld.Currency == nil {
			amld.Currency = h.Currency().NewSet(amlr.Env())
		}
		return amlr.Account().Equals(amld.Account) &&
			amlr.Currency().Equals(amld.Currency) &&
			nbutils.Round(amlr.Debit(), 0.01) == amld.Debit &&
			nbutils.Round(amlr.Credit(), 0.01) == amld.Credit &&
			nbutils.Round(amlr.AmountCurrency(), 0.01) == amld.AmountCurrency
	}

	for _, amlData := range amlDatas {
		// There is no unique key to identify journal items (an account_payment may create several lines
		// in the same account), so to check the expected entries are created, we check there is a line
		// matching for each dict of expected values
		amlRec := amlRecs.Filtered(func(set m.AccountMoveLineSet) bool {
			return compareRecData(set, amlData)
		})
		So(amlRec.Len(), ShouldEqual, 1)
		if amlData.CurrencyDiff != 0 {
			currencyDiffMove := amlRec.MatchedCredits().FullReconcile().ExchangeMove()
			if amlRec.Credit() != 0 {
				currencyDiffMove = amlRec.MatchedDebits().FullReconcile().ExchangeMove()
			}
			for _, currencyDiffLine := range currencyDiffMove.Lines().Records() {
				if amlData.CurrencyDiff > 0 {
					if currencyDiffLine.Account().Equals(amlRec.Account()) {
						So(currencyDiffLine.Debit(), ShouldAlmostEqual, amlData.CurrencyDiff, 0.0000001)
					} else {
						So(currencyDiffLine.Credit(), ShouldAlmostEqual, amlData.CurrencyDiff, 0.0000001)
						So(currencyDiffLine.Account().Equals(tps.DiffIncomeAccount) ||
							currencyDiffLine.Account().Equals(tps.DiffExpenseAccount), ShouldBeTrue)
					}
				} else {
					if currencyDiffLine.Account().Equals(amlRec.Account()) {
						So(currencyDiffLine.Credit(), ShouldAlmostEqual, amlData.CurrencyDiff, 0.0000001)
					} else {
						So(currencyDiffLine.Debit(), ShouldAlmostEqual, amlData.CurrencyDiff, 0.0000001)
						So(currencyDiffLine.Account().Equals(tps.DiffIncomeAccount) ||
							currencyDiffLine.Account().Equals(tps.DiffExpenseAccount), ShouldBeTrue)
					}

				}
			}
		}
	}
}

func TestFullPaymentProcess(t *testing.T) {
	Convey("Test Full Payment Process", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			tps := initTestPaymentStruct(env)

			inv1 := tps.CreateInvoice(100, "", tps.CurrencyEur)
			inv2 := tps.CreateInvoice(200, "", tps.CurrencyEur)
			So(inv1.State(), ShouldEqual, "open")
			So(inv2.State(), ShouldEqual, "open")

			ctx := env.Context().
				WithKey("active_model", "account.invoice").
				WithKey("active_ids", []int64{inv1.ID(), inv2.ID()})
			registerPaymentsData := h.AccountRegisterPayments().NewSet(env).WithNewContext(ctx).DefaultGet().
				SetPaymentDate(dates.ParseDate("2015-07-15")).
				SetJournal(tps.BankJournalEuro).
				SetPaymentMethod(tps.PaymentMethodManualIn)
			registerPayments := h.AccountRegisterPayments().NewSet(env).WithNewContext(ctx).Create(registerPaymentsData)
			registerPayments.CreatePayment()
			payment := h.AccountPayment().NewSet(env).SearchAll().OrderBy("id desc").Limit(1)

			So(payment.Amount(), ShouldAlmostEqual, 300, 0.0000001)
			So(payment.State(), ShouldEqual, "posted")
			So(inv1.State(), ShouldEqual, "paid")
			So(inv2.State(), ShouldEqual, "paid")
			tps.CheckJournalItems(payment.MoveLines(), []TestAMLStruct{
				{Account: tps.AccountEur, Debit: 300},
				{Account: inv1.Account(), Credit: 300},
			})

			liquidityAml := payment.MoveLines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.Account().Equals(tps.AccountEur)
			})
			bankStatement := tps.Reconcile(liquidityAml, 200, 0, h.Currency().NewSet(env))

			So(liquidityAml.Statement().Equals(bankStatement), ShouldBeTrue)
			So(liquidityAml.Move().StatementLine().Equals(bankStatement.Lines().Records()[0]), ShouldBeTrue)
			So(payment.State(), ShouldEqual, "reconciled")
		}), ShouldBeNil)
	})
}

func TestInternalTransferJournalUsdJournalEur(t *testing.T) {
	Convey("Test transfer journal usd->eur", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			tps := initTestPaymentStruct(env)
			payment := h.AccountPayment().Create(env,
				h.AccountPayment().NewData().
					SetPaymentDate(dates.ParseDate("2015-07-15")).
					SetPaymentType("transfer").
					SetAmount(50).
					SetCurrency(tps.CurrencyUsd).
					SetJournal(tps.BankJournalUsd).
					SetDestinationJournal(tps.BankJournalEuro).
					SetPaymentMethod(tps.PaymentMethodManualOut))
			payment.Post()
			tps.CheckJournalItems(payment.MoveLines(), []TestAMLStruct{
				{Account: tps.TransferAccount, Debit: 32.70, AmountCurrency: 50, Currency: tps.CurrencyUsd},
				{Account: tps.TransferAccount, Credit: 32.70, AmountCurrency: -50, Currency: tps.CurrencyUsd},
				{Account: tps.AccountEur, Debit: 32.70, AmountCurrency: 0},
				{Account: tps.AccountUsd, Credit: 32.70, AmountCurrency: -50, Currency: tps.CurrencyUsd},
			})
		}), ShouldBeNil)
	})
}

func TestPaymentChfJournalUsd(t *testing.T) {
	Convey("Test payment chf journal usd", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			tps := initTestPaymentStruct(env)
			payment := h.AccountPayment().Create(env,
				h.AccountPayment().NewData().
					SetPaymentDate(dates.ParseDate("2015-07-15")).
					SetPaymentType("outbound").
					SetAmount(50).
					SetCurrency(tps.CurrencyChf).
					SetJournal(tps.BankJournalUsd).
					SetPartnerType("supplier").
					SetPartner(tps.PartnerAxelor).
					SetPaymentMethod(tps.PaymentMethodManualOut))
			payment.Post()

			tps.CheckJournalItems(payment.MoveLines(), []TestAMLStruct{
				{Account: tps.AccountUsd, Credit: 38.21, AmountCurrency: -58.42, Currency: tps.CurrencyUsd},
				{Account: tps.PartnerAxelor.PropertyAccountPayable(), Debit: 38.21, AmountCurrency: 50, Currency: tps.CurrencyChf},
			})
		}), ShouldBeNil)
	})
}
