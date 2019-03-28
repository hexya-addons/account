package account

import (
	"fmt"
	"testing"

	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
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
func (self TestPaymentStruct) CreateInvoice(amount float64, typ string, currency m.CurrencySet) m.AccountInvoiceSet {
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

	invoice := h.AccountInvoice().Create(self.Env, h.AccountInvoice().NewData().
		SetPartner(self.PartnerAgrolait).
		SetReferenceType("none").
		SetCurrency(currency).
		SetName(invoiceName).
		SetAccount(self.AccountReceivable).
		SetType(typ).
		SetDate(dates.Today().SetMonth(6).SetDay(26)))

	h.AccountInvoiceLine().Create(self.Env, h.AccountInvoiceLine().NewData().
		SetProduct(self.Product).
		SetQuantity(1).
		SetPriceUnit(amount).
		SetInvoice(invoice).
		SetName("something").
		SetAccount(self.AccountRevenue))

	invoice.ActionInvoiceOpen()
	return invoice

}

// Reconcile reconcile a journal entry corresponding to a payment with its bank statement line
func (self TestPaymentStruct) Reconcile(liquidityAml m.AccountMoveLineSet, amount, amountCurrency float64, currency m.CurrencySet) m.AccountBankStatementSet {
	date := dates.Today().SetMonth(7).SetDay(15)
	bankStmt := h.AccountBankStatement().Create(self.Env, h.AccountBankStatement().NewData().
		SetJournal(liquidityAml.Journal()).
		SetDate(date))
	bankStmtLine := h.AccountBankStatementLine().Create(self.Env, h.AccountBankStatementLine().NewData().
		SetName("payment").
		SetStatement(bankStmt).
		SetPartner(self.PartnerAgrolait).
		SetAmount(amount).
		SetAmountCurrency(amountCurrency).
		SetCurrency(currency).
		SetDate(date))
	bankStmtLine.ProcessReconciliation(h.AccountMoveLine().NewSet(self.Env), liquidityAml.All(), h.AccountMoveLine().NewSet(self.Env).All())
	return bankStmt
}

func (self TestPaymentStruct) CheckJournalItems(amlRecs m.AccountMoveLineSet, amlDatas []m.AccountMoveLineData) {
	compareRecData := func(amlRec m.AccountMoveLineSet, amlData m.AccountMoveLineData) bool {
		return amlRec.Account().Equals(amlData.Account()) &&
			amlRec.Currency().Equals(amlData.Currency()) &&
			nbutils.Round(amlRec.Debit(), 0.01) == amlData.Debit() &&
			nbutils.Round(amlRec.Credit(), 0.01) == amlData.Credit() &&
			nbutils.Round(amlRec.AmountCurrency(), 0.01) == amlData.AmountCurrency()
	}

	for _, amlData := range amlDatas {
		// There is no unique key to identify journal items (an account_payment may create several lines
		// in the same account), so to check the expected entries are created, we check there is a line
		// matching for each dict of expected values
		amlRec := amlRecs.Filtered(func(set m.AccountMoveLineSet) bool {
			return compareRecData(set, amlData)
		})
		errorText := ""
		if amlRec.Len() != 1 {
			errorText = fmt.Sprintf("Expected one move line with values: %#v", amlData.Underlying())
		}
		So(errorText, ShouldEqual, "")
		/* this piece of code (odoo) does not seems to be used anywhere
		   if aml_dict.get('currency_diff'):
		       if aml_rec.credit:
		           currency_diff_move = aml_rec.matched_debit_ids.full_reconcile_id.exchange_move_id
		       else:
		           currency_diff_move = aml_rec.matched_credit_ids.full_reconcile_id.exchange_move_id
		       for currency_diff_line in currency_diff_move.line_ids:
		           if aml_dict.get('currency_diff') > 0:
		               if currency_diff_line.account_id.id == aml_rec.account_id.id:
		                   self.assertAlmostEquals(currency_diff_line.debit, aml_dict.get('currency_diff'))
		               else:
		                   self.assertAlmostEquals(currency_diff_line.credit, aml_dict.get('currency_diff'))
		                   self.assertIn(currency_diff_line.account_id.id, [self.diff_expense_account.id, self.diff_income_account.id])
		           else:
		               if currency_diff_line.account_id.id == aml_rec.account_id.id:
		                   self.assertAlmostEquals(currency_diff_line.credit, abs(aml_dict.get('currency_diff')))
		               else:
		                   self.assertAlmostEquals(currency_diff_line.debit, abs(aml_dict.get('currency_diff')))
		                   self.assertIn(currency_diff_line.account_id.id, [self.diff_expense_account.id, self.diff_income_account.id])
		*/
	}
}

func TestPayment(t *testing.T) {
	Convey("Tests Payment", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			self := initTestPaymentStruct(env)
			Convey("Test full payment process", func() {
				inv1 := self.CreateInvoice(100, "", self.CurrencyEur)
				inv2 := self.CreateInvoice(200, "", self.CurrencyEur)

				ctx := types.NewContext().
					WithKey("active_model", "account.invoice").
					WithKey("active_ids", []int64{inv1.ID(), inv2.ID()})
				registerPayments := h.AccountRegisterPayments().NewSet(env).WithNewContext(ctx).Create(
					h.AccountRegisterPayments().NewData().
						SetPaymentDate(dates.Today().SetMonth(7).SetDay(15)).
						SetJournal(self.BankJournalEuro).
						SetPaymentMethod(self.PaymentMethodManualIn))
				registerPayments.CreatePayment()
				payment := h.AccountPayment().Search(env, q.AccountPayment().ID().Greater(-1)).OrderBy("id desc").Limit(1)

				So(payment.Amount(), ShouldEqual, 300)
				So(payment.State(), ShouldEqual, "posted")
				So(inv1, ShouldEqual, "paid")
				So(inv2, ShouldEqual, "paid")
				self.CheckJournalItems(payment.MoveLines(), []m.AccountMoveLineData{
					h.AccountMoveLine().NewData().SetAccount(self.AccountEur).SetDebit(300),
					h.AccountMoveLine().NewData().SetAccount(inv1.Account()).SetCredit(300),
				})

				liquidityAml := payment.MoveLines().Filtered(func(set m.AccountMoveLineSet) bool {
					return set.Account().Equals(self.AccountEur)
				})
				bankStatement := self.Reconcile(liquidityAml, 200, 0, h.Currency().NewSet(env))

				So(liquidityAml.Statement().Equals(bankStatement), ShouldBeTrue)
				So(liquidityAml.Move().StatementLine().Equals(bankStatement.Lines().Records()[0]), ShouldBeTrue)
				So(payment.State(), ShouldEqual, "reconciled")
			})
			Convey("Test transfer journal usd->eur", func() {
				payment := h.AccountPayment().Create(env,
					h.AccountPayment().NewData().
						SetPaymentDate(dates.Today().SetMonth(7).SetDay(15)).
						SetPaymentType("transfer").
						SetAmount(50).
						SetCurrency(self.CurrencyUsd).
						SetJournal(self.BankJournalUsd).
						SetDestinationJournal(self.BankJournalEuro).
						SetPaymentMethod(self.PaymentMethodManualOut))
				payment.Post()
				self.CheckJournalItems(payment.MoveLines(), []m.AccountMoveLineData{
					h.AccountMoveLine().NewData().SetAccount(self.TransferAccount).SetDebit(32.70).SetAmountCurrency(50).SetCurrency(self.CurrencyUsd),
					h.AccountMoveLine().NewData().SetAccount(self.TransferAccount).SetCredit(32.70).SetAmountCurrency(-50).SetCurrency(self.CurrencyUsd),
					h.AccountMoveLine().NewData().SetAccount(self.AccountEur).SetDebit(32.70),
					h.AccountMoveLine().NewData().SetAccount(self.AccountUsd).SetCredit(32.70).SetAmountCurrency(-50).SetCurrency(self.CurrencyUsd),
				})
			})
			Convey("Test payment chf journal usd", func() {
				payment := h.AccountPayment().Create(env,
					h.AccountPayment().NewData().
						SetPaymentDate(dates.Today().SetMonth(7).SetDay(15)).
						SetPaymentType("outbound").
						SetAmount(50).
						SetCurrency(self.CurrencyChf).
						SetJournal(self.BankJournalUsd).
						SetPartnerType("supplier").
						SetPartner(self.PartnerAxelor).
						SetPaymentMethod(self.PaymentMethodManualOut))
				payment.Post()

				self.CheckJournalItems(payment.MoveLines(), []m.AccountMoveLineData{
					h.AccountMoveLine().NewData().SetAccount(self.AccountUsd).SetCredit(38.21),
					h.AccountMoveLine().NewData().SetAccount(self.PartnerAxelor.PropertyAccountPayable()).SetDebit(38.21).SetAmountCurrency(50).SetCurrency(self.CurrencyChf),
				})
			})
		}), ShouldBeNil)
	})
}
