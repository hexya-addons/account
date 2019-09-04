package account

import (
	"fmt"
	"math"
	"testing"

	"github.com/hexya-addons/account/accounttypes"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
	. "github.com/smartystreets/goconvey/convey"
)

/*
   Tests for reconciliation (account.tax)

   Test used to check that when doing a sale or purchase invoice in a different currency,
   the result will be balanced.
*/

type TestReconciliationStruct struct {
	TestAccountBaseStruct

	CurrentUser           m.UserSet
	PartnerAgrolait       m.PartnerSet
	CurrencySwiss         m.CurrencySet
	CurrencyUsd           m.CurrencySet
	CurrencyEuro          m.CurrencySet
	CurrencyFalse         m.CurrencySet
	AccountRcv            m.AccountAccountSet
	AccountRsa            m.AccountAccountSet
	AccountEuro           m.AccountAccountSet
	DiffIncomeAccount     m.AccountAccountSet
	DiffExpenseAccount    m.AccountAccountSet
	AccountUsd            m.AccountAccountSet
	BankJournalEuro       m.AccountJournalSet
	BankJournalUsd        m.AccountJournalSet
	Product               m.ProductProductSet
	AccountTypeReceivable m.AccountAccountTypeSet
	AccountTypePayable    m.AccountAccountTypeSet
	AccountTypeRevenue    m.AccountAccountTypeSet
	AccountTypeExpenses   m.AccountAccountTypeSet
	InboundPaymentMethod  m.AccountPaymentMethodSet
}

func initTestReconciliationStruct(env models.Environment) TestReconciliationStruct {
	var out TestReconciliationStruct
	out.TestAccountBaseStruct = initTestAccountBaseStruct(env)
	out.CurrencyFalse = h.Currency().NewSet(env)

	out.PartnerAgrolait = h.Partner().NewSet(env).GetRecord("base_res_partner_2")
	out.CurrencySwiss = h.Currency().NewSet(env).GetRecord("base_CHF")
	out.CurrencySwiss.SetActive(true)
	out.CurrencyUsd = h.Currency().NewSet(env).GetRecord("base_USD")
	out.CurrencyEuro = h.Currency().NewSet(env).GetRecord("base_EUR")

	out.AccountTypeReceivable = h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_receivable")
	out.AccountTypePayable = h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_payable")
	out.AccountTypeRevenue = h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_revenue")
	out.AccountTypeExpenses = h.AccountAccountType().NewSet(env).GetRecord("account_data_account_type_expenses")

	cmpny := h.Company().NewSet(env).GetRecord("base_main_company")
	cmpny.SetCurrency(out.CurrencyEuro)

	out.AccountRcv = out.PartnerAgrolait.PropertyAccountReceivable()
	if out.AccountRcv.IsEmpty() {
		out.AccountRcv = h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(out.AccountTypeReceivable))
	}
	out.AccountRsa = out.PartnerAgrolait.PropertyAccountPayable()
	if out.AccountRsa.IsEmpty() {
		out.AccountRsa = h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(out.AccountTypePayable))
	}

	out.Product = h.ProductProduct().NewSet(env).GetRecord("product_product_product_4")

	journalEuro := h.AccountJournal().NewData().
		SetName("Bank").
		SetType("bank").
		SetCode("BNK67")
	JournalUsd := h.AccountJournal().NewData().
		SetName("Bank US").
		SetType("bank").
		SetCode("BNK68").
		SetCurrency(out.CurrencyUsd)
	out.BankJournalEuro = h.AccountJournal().Create(env, journalEuro)
	out.BankJournalUsd = h.AccountJournal().Create(env, JournalUsd)
	out.AccountEuro = out.BankJournalEuro.DefaultDebitAccount()
	out.AccountUsd = out.BankJournalUsd.DefaultDebitAccount()

	out.CurrentUser = h.User().NewSet(env).CurrentUser()

	out.DiffIncomeAccount = out.CurrentUser.Company().IncomeCurrencyExchangeAccount()
	out.DiffExpenseAccount = out.CurrentUser.Company().ExpenseCurrencyExchangeAccount()

	out.InboundPaymentMethod = h.AccountPaymentMethod().Create(env,
		h.AccountPaymentMethod().NewData().
			SetName("inbound").
			SetCode("IN").
			SetPaymentType("inbound"))

	return out
}

func (trs TestReconciliationStruct) createInvoice(env models.Environment, typ string, amount float64, currency m.CurrencySet) m.AccountInvoiceSet {
	return trs.createInvoicePartner(env, typ, amount, currency, trs.PartnerAgrolait)
}

func (trs TestReconciliationStruct) createInvoicePartner(env models.Environment, typ string, amount float64, currency m.CurrencySet, partner m.PartnerSet) m.AccountInvoiceSet {
	// we create an invoice in given currency
	name := "invoice to vendor"
	if typ == "out_invoice" {
		name = "invoice to client"
	}
	invoice := h.AccountInvoice().Create(env,
		h.AccountInvoice().NewData().
			SetPartner(partner).
			SetReferenceType("none").
			SetCurrency(currency).
			SetName(name).
			SetAccount(trs.AccountRcv).
			SetType(typ).
			SetDateInvoice(dates.ParseDate("2015-07-01")))
	h.AccountInvoiceLine().Create(env,
		h.AccountInvoiceLine().NewData().
			SetProduct(trs.Product).
			SetQuantity(1).
			SetPriceUnit(amount).
			SetInvoice(invoice).
			SetName(fmt.Sprintf("product that cost %f", amount)).
			SetAccount(h.AccountAccount().Search(env, q.AccountAccount().UserType().Equals(trs.AccountTypeRevenue))))
	// validate invoice
	invoice.ActionInvoiceOpen()
	return invoice
}

func (trs TestReconciliationStruct) makePayment(env models.Environment, invoice m.AccountInvoiceSet, bankJournal m.AccountJournalSet,
	amount, amountCurrency float64, currency m.CurrencySet) m.AccountBankStatementSet {
	bankStmt := h.AccountBankStatement().Create(env,
		h.AccountBankStatement().NewData().
			SetJournal(bankJournal).
			SetDate(dates.ParseDate("2015-07-15")).
			SetName("payment"+invoice.Number()))
	bankStmtLine := h.AccountBankStatementLine().Create(env,
		h.AccountBankStatementLine().NewData().
			SetName("payment").
			SetStatement(bankStmt).
			SetPartner(trs.PartnerAgrolait).
			SetAmount(amount).
			SetAmountCurrency(amountCurrency).
			SetCurrency(currency).
			SetDate(dates.ParseDate("2015-07-15")))

	// reconcile the payment with the invoice
	line := h.AccountMoveLine().NewSet(env)
	for _, l := range invoice.Move().Lines().Records() {
		if l.Account().Equals(trs.AccountRcv) {
			line = l
			break
		}
	}
	amountInWidget := amount
	if currency.IsNotEmpty() {
		amountInWidget = amountCurrency
	}

	data := accounttypes.BankStatementAMLStruct{
		MoveLineID: line.ID(),
		Debit:      0.0,
		Credit:     0.0,
		Name:       line.Name(),
	}
	if amountInWidget < 0 {
		data.Debit = -amountInWidget
	} else {
		data.Credit = amountInWidget
	}
	bankStmtLine.ProcessReconciliation(h.AccountMoveLine().NewSet(env), []accounttypes.BankStatementAMLStruct{data}, nil)
	return bankStmt
}

type amlStruct struct {
	debit              float64
	credit             float64
	amountCurrency     float64
	currency           m.CurrencySet
	currencyDiff       float64
	amountCurrencyDiff float64
	hasCurrencyDiff    bool
}

type amlMap map[int64]amlStruct

func (trs TestReconciliationStruct) checkResults(moveLineRecs m.AccountMoveLineSet, amlDict amlMap) {
	// we check that the line is balanced (bank statement line)
	So(moveLineRecs.Len(), ShouldEqual, len(amlDict))

	for _, moveLine := range moveLineRecs.Records() {
		aml := amlDict[moveLine.Account().ID()]
		So(nbutils.Round(moveLine.Debit(), 0.01), ShouldEqual, aml.debit)
		So(nbutils.Round(moveLine.Credit(), 0.01), ShouldEqual, aml.credit)
		So(nbutils.Round(moveLine.AmountCurrency(), 0.01), ShouldEqual, aml.amountCurrency)
		So(moveLine.Currency().Equals(aml.currency), ShouldBeTrue)
		if !(aml.currencyDiff != 0 || aml.hasCurrencyDiff) {
			continue
		}
		currencyDiffMove := moveLine.FullReconcile().ExchangeMove()
		for _, line := range currencyDiffMove.Lines().Records() {

			if aml.currencyDiff == 0 && line.Account().Equals(moveLine.Account()) {
				So(line.AmountCurrency(), ShouldAlmostEqual, aml.amountCurrencyDiff)
			}
			if aml.currencyDiff > 0 {
				if line.Account().Equals(moveLine.Account()) {
					So(line.Debit(), ShouldAlmostEqual, aml.currencyDiff)
				} else {
					So(line.Credit(), ShouldAlmostEqual, aml.currencyDiff)
					So(line.Account().ID(), ShouldBeIn, []int64{trs.DiffExpenseAccount.ID(), trs.DiffIncomeAccount.ID()})
				}
			} else {
				if line.Account().Equals(moveLine.Account()) {
					So(line.Credit(), ShouldAlmostEqual, math.Abs(aml.currencyDiff))
				} else {
					So(line.Debit(), ShouldAlmostEqual, math.Abs(aml.currencyDiff))
					So(line.Account().ID(), ShouldBeIn, []int64{trs.DiffExpenseAccount.ID(), trs.DiffIncomeAccount.ID()})
				}
			}
		}
	}
}

func (trs TestReconciliationStruct) makeCustomerAndSupplierFlows(env models.Environment,
	invoiceCurrency m.CurrencySet, invoiceAmount float64,
	bankJournal m.AccountJournalSet, amount, amountCurrency float64,
	transactionCurrency m.CurrencySet) (m.AccountMoveLineSet, m.AccountMoveLineSet) {
	// we create an invoice in given invoice_currency
	invoiceRecord := trs.createInvoice(env, "out_invoice", invoiceAmount, invoiceCurrency)
	// we encode a payment on it, on the given bank_journal with amount, amount_currency and transaction_currency given
	bankStmt := trs.makePayment(env, invoiceRecord, bankJournal, amount, amountCurrency, transactionCurrency)
	customerMoveLines := bankStmt.MoveLines()

	// we create a supplier bill in given invoice_currency
	invoiceRecord = trs.createInvoice(env, "in_invoice", invoiceAmount, invoiceCurrency)
	// we encode a payment on it, on the given bank_journal with amount, amount_currency and transaction_currency given
	bankStmt = trs.makePayment(env, invoiceRecord, bankJournal, -amount, -amountCurrency, transactionCurrency)
	supplierMoveLines := bankStmt.MoveLines()
	return customerMoveLines, supplierMoveLines
}

type moveLineStruct struct {
	name           string
	amount         float64
	amountCurrency float64
	currency       m.CurrencySet
}

func (trs TestReconciliationStruct) createMove(env models.Environment, lineStruct moveLineStruct) m.AccountMoveSet {
	debitLineVals := h.AccountMoveLine().NewData().
		SetName(lineStruct.name).
		SetDebit(0).
		SetCredit(0).
		SetAccount(trs.AccountRcv).
		SetAmountCurrency(lineStruct.amountCurrency).
		SetCurrency(lineStruct.currency)
	if lineStruct.amount > 0 {
		debitLineVals.SetDebit(lineStruct.amount)
	} else {
		debitLineVals.SetCredit(-lineStruct.amount)
	}

	creditLineVals := debitLineVals.Copy().
		SetDebit(debitLineVals.Credit()).
		SetCredit(debitLineVals.Debit()).
		SetAccount(trs.AccountRsa).
		SetAmountCurrency(-debitLineVals.AmountCurrency())

	return h.AccountMove().Create(env,
		h.AccountMove().NewData().
			SetJournal(trs.BankJournalEuro).
			CreateLines(debitLineVals).
			CreateLines(creditLineVals))
}

func (trs TestReconciliationStruct) determineDebitCreditLine(move m.AccountMoveSet) []m.AccountMoveLineSet {
	lines := move.Lines().Filtered(func(set m.AccountMoveLineSet) bool {
		return set.Account().Reconcile() || set.Account().InternalType() == "liquidity"
	})
	ret1 := lines.Filtered(func(set m.AccountMoveLineSet) bool {
		return set.Debit() != 0
	})
	ret2 := lines.Filtered(func(set m.AccountMoveLineSet) bool {
		return set.Credit() != 0
	})
	out := []m.AccountMoveLineSet{ret1, ret2}
	return out
}

func (trs TestReconciliationStruct) moveRevertTestPair(move, revert m.AccountMoveSet) {
	So(move.Lines().IsNotEmpty(), ShouldBeTrue)
	So(revert.Lines().IsNotEmpty(), ShouldBeTrue)

	movelines := trs.determineDebitCreditLine(move)
	revertLines := trs.determineDebitCreditLine(revert)

	//in the case of the exchange entry, only one pair of lines will be found

	if movelines[0].IsNotEmpty() && revertLines[1].IsNotEmpty() {
		So(movelines[0].FullReconcile().IsNotEmpty(), ShouldBeTrue)
		So(movelines[0].FullReconcile().Equals(revertLines[1].FullReconcile()), ShouldBeTrue)
	}
	if movelines[1].IsNotEmpty() && revertLines[0].IsNotEmpty() {
		So(movelines[1].FullReconcile().IsNotEmpty(), ShouldBeTrue)
		So(movelines[1].FullReconcile().Equals(revertLines[0].FullReconcile()), ShouldBeTrue)
	}
}

func TestStatementUsdInvoiceEurTransactionEur(t *testing.T) {
	Convey("Test Statement Uset Invoice Eur Transaction Eur", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			customerMoveLines, supplierMoveLines := trs.makeCustomerAndSupplierFlows(env, trs.CurrencyEuro,
				30, trs.BankJournalUsd, 42, 30, trs.CurrencyEuro)
			trs.checkResults(customerMoveLines, amlMap{
				trs.AccountUsd.ID(): amlStruct{debit: 30, credit: 0, amountCurrency: 42, currency: trs.CurrencyUsd},
				trs.AccountRcv.ID(): amlStruct{debit: 0, credit: 30, amountCurrency: -42, currency: trs.CurrencyUsd},
			})
			trs.checkResults(supplierMoveLines, amlMap{
				trs.AccountUsd.ID(): amlStruct{debit: 0, credit: 30, amountCurrency: -42, currency: trs.CurrencyUsd},
				trs.AccountRcv.ID(): amlStruct{debit: 30, credit: 0, amountCurrency: 42, currency: trs.CurrencyUsd},
			})
		}), ShouldBeNil)
	})
}

func TestStatementUsdInvoiceUsdTransactionUsd(t *testing.T) {
	Convey("Test statement usd invoice usd transaction usd", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			customerMoveLines, supplierMoveLines := trs.makeCustomerAndSupplierFlows(env, trs.CurrencyUsd,
				50, trs.BankJournalUsd, 50, 0, trs.CurrencyFalse)
			trs.checkResults(customerMoveLines, amlMap{
				trs.AccountUsd.ID(): amlStruct{debit: 32.7, credit: 0, amountCurrency: 50, currency: trs.CurrencyUsd},
				trs.AccountRcv.ID(): amlStruct{debit: 0, credit: 32.7, amountCurrency: -50, currency: trs.CurrencyUsd},
			})
			trs.checkResults(supplierMoveLines, amlMap{
				trs.AccountUsd.ID(): amlStruct{debit: 0, credit: 32.7, amountCurrency: -50, currency: trs.CurrencyUsd},
				trs.AccountRcv.ID(): amlStruct{debit: 32.7, credit: 0, amountCurrency: 50, currency: trs.CurrencyUsd},
			})
		}), ShouldBeNil)
	})
}

func TestStatementUsdInvoiceUsdTransactionEur(t *testing.T) {
	Convey("Test statement usd invoice usd transaction eur", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			customerMoveLines, supplierMoveLines := trs.makeCustomerAndSupplierFlows(env, trs.CurrencyUsd,
				50, trs.BankJournalUsd, 50, 40, trs.CurrencyEuro)
			trs.checkResults(customerMoveLines, amlMap{
				trs.AccountUsd.ID(): amlStruct{debit: 40, credit: 0, amountCurrency: 50, currency: trs.CurrencyUsd},
				trs.AccountRcv.ID(): amlStruct{debit: 0, credit: 40, amountCurrency: -50, currency: trs.CurrencyUsd, currencyDiff: 7.30},
			})
			trs.checkResults(supplierMoveLines, amlMap{
				trs.AccountUsd.ID(): amlStruct{debit: 0, credit: 40, amountCurrency: -50, currency: trs.CurrencyUsd},
				trs.AccountRcv.ID(): amlStruct{debit: 40, credit: 0, amountCurrency: 50, currency: trs.CurrencyUsd, currencyDiff: -7.30},
			})
		}), ShouldBeNil)
	})
}

func TestStatementUsdInvoiceChfTransactionChf(t *testing.T) {
	Convey("Test statement usd invoice chf transaction chf", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			customerMoveLines, supplierMoveLines := trs.makeCustomerAndSupplierFlows(env, trs.CurrencySwiss,
				50, trs.BankJournalUsd, 42, 50, trs.CurrencySwiss)
			trs.checkResults(customerMoveLines, amlMap{
				trs.AccountUsd.ID(): amlStruct{debit: 27.47, credit: 0, amountCurrency: 42, currency: trs.CurrencyUsd},
				trs.AccountRcv.ID(): amlStruct{debit: 0, credit: 27.47, amountCurrency: -50, currency: trs.CurrencySwiss, currencyDiff: -10.74},
			})
			trs.checkResults(supplierMoveLines, amlMap{
				trs.AccountUsd.ID(): amlStruct{debit: 0, credit: 27.47, amountCurrency: -42, currency: trs.CurrencyUsd},
				trs.AccountRcv.ID(): amlStruct{debit: 27.47, credit: 0, amountCurrency: 50, currency: trs.CurrencySwiss, currencyDiff: 10.74},
			})
		}), ShouldBeNil)
	})
}

func TestStatementEurInvoiceUsdTransactionUsd(t *testing.T) {
	Convey("Test statement eur invoice usd transaction usd", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			customerMoveLines, supplierMoveLines := trs.makeCustomerAndSupplierFlows(env, trs.CurrencyUsd,
				50, trs.BankJournalEuro, 40, 50, trs.CurrencyUsd)
			trs.checkResults(customerMoveLines, amlMap{
				trs.AccountEuro.ID(): amlStruct{debit: 40, credit: 0, amountCurrency: 50, currency: trs.CurrencyUsd},
				trs.AccountRcv.ID():  amlStruct{debit: 0, credit: 40, amountCurrency: -50, currency: trs.CurrencyUsd, currencyDiff: 7.30},
			})
			trs.checkResults(supplierMoveLines, amlMap{
				trs.AccountEuro.ID(): amlStruct{debit: 0, credit: 40, amountCurrency: -50, currency: trs.CurrencyUsd},
				trs.AccountRcv.ID():  amlStruct{debit: 40, credit: 0, amountCurrency: 50, currency: trs.CurrencyUsd, currencyDiff: -7.30},
			})
		}), ShouldBeNil)
	})
}

func TestStatementEurInvoiceUsdTransactionEur(t *testing.T) {
	Convey("Test statement eur invoice usd transaction eur", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			customerMoveLines, supplierMoveLines := trs.makeCustomerAndSupplierFlows(env, trs.CurrencyUsd,
				50, trs.BankJournalEuro, 40, 0, trs.CurrencyFalse)
			trs.checkResults(customerMoveLines, amlMap{
				trs.AccountEuro.ID(): amlStruct{debit: 40, credit: 0, amountCurrency: 0, currency: trs.CurrencyFalse},
				trs.AccountRcv.ID():  amlStruct{debit: 0, credit: 40, amountCurrency: -61.16, currency: trs.CurrencyUsd},
			})
			trs.checkResults(supplierMoveLines, amlMap{
				trs.AccountEuro.ID(): amlStruct{debit: 0, credit: 40, amountCurrency: -0, currency: trs.CurrencyFalse},
				trs.AccountRcv.ID():  amlStruct{debit: 40, credit: 0, amountCurrency: 61.16, currency: trs.CurrencyUsd},
			})
		}), ShouldBeNil)
	})
}

func TestStatementEurInvoiceUsdTransactionChf(t *testing.T) {
	Convey("Test statement eur invoice usd transaction chf", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			customerMoveLines, supplierMoveLines := trs.makeCustomerAndSupplierFlows(env, trs.CurrencyUsd,
				50, trs.BankJournalEuro, 42, 50, trs.CurrencySwiss)
			trs.checkResults(customerMoveLines, amlMap{
				trs.AccountEuro.ID(): amlStruct{debit: 42, credit: 0, amountCurrency: 50, currency: trs.CurrencySwiss},
				trs.AccountRcv.ID():  amlStruct{debit: 0, credit: 42, amountCurrency: -50, currency: trs.CurrencySwiss},
			})
			trs.checkResults(supplierMoveLines, amlMap{
				trs.AccountEuro.ID(): amlStruct{debit: 0, credit: 42, amountCurrency: -50, currency: trs.CurrencySwiss},
				trs.AccountRcv.ID():  amlStruct{debit: 42, credit: 0, amountCurrency: 50, currency: trs.CurrencySwiss},
			})
		}), ShouldBeNil)
	})
}

func TestStatementEurInvoiceUsdTransactionEuroFull(t *testing.T) {
	Convey("test_statement_euro_invoice_usd_transaction_euro_full", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			// we create an invoice in given invoice_currency
			invoiceRecord := trs.createInvoice(env, "out_invoice", 50, trs.CurrencyUsd)
			// we encode a payment on it, on the given bank_journal with amount, amount_currency and transaction_currency given
			bankStmt := h.AccountBankStatement().Create(env,
				h.AccountBankStatement().NewData().
					SetJournal(trs.BankJournalEuro).
					SetDate(dates.ParseDate("2015-01-01")))
			bankStmtLine := h.AccountBankStatementLine().Create(env,
				h.AccountBankStatementLine().NewData().
					SetName("payment").
					SetStatement(bankStmt).
					SetPartner(trs.PartnerAgrolait).
					SetAmount(40).
					SetDate(dates.ParseDate("2015-01-01")))

			// reconcile the payment with the invoice
			line := h.AccountMoveLine().NewSet(env)
			for _, l := range invoiceRecord.Move().Lines().Records() {
				if l.Account().Equals(trs.AccountRcv) {
					line = l
					break
				}
			}
			bankStmtLine.ProcessReconciliation(h.AccountMoveLine().NewSet(env),
				[]accounttypes.BankStatementAMLStruct{{
					MoveLineID: line.ID(),
					Debit:      0,
					Credit:     32.7,
					Name:       line.Name(),
				}},
				[]accounttypes.BankStatementAMLStruct{{
					Debit:     0,
					Credit:    7.3,
					Name:      "Exchange Difference",
					AccountID: trs.DiffIncomeAccount.ID(),
				}})

			trs.checkResults(bankStmt.MoveLines(), amlMap{
				trs.AccountEuro.ID():       amlStruct{debit: 40, credit: 0, amountCurrency: 0, currency: trs.CurrencyFalse},
				trs.AccountRcv.ID():        amlStruct{debit: 0, credit: 32.7, amountCurrency: -41.97, currency: trs.CurrencyUsd, currencyDiff: 0, hasCurrencyDiff: true, amountCurrencyDiff: -8.03},
				trs.DiffIncomeAccount.ID(): amlStruct{debit: 0, credit: 7.3, amountCurrency: -9.37, currency: trs.CurrencyUsd},
			})

			// The invoice should be paid, as the payments totally cover its total
			So(invoiceRecord.State(), ShouldEqual, "paid")
			invoiceRecLine := invoiceRecord.Move().Lines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.Account().Reconcile()
			})
			So(invoiceRecLine.Reconciled(), ShouldBeTrue)
			So(invoiceRecLine.AmountResidual(), ShouldEqual, 0)
			So(invoiceRecLine.AmountResidualCurrency(), ShouldEqual, 0)

		}), ShouldBeNil)
	})
}

func TestManualReconcileWizardOpw678153(t *testing.T) {
	Convey("Test manual_reconcile_wizard_opw678153", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			moveListVals := []moveLineStruct{
				{name: "1", amount: -1.83, amountCurrency: 0, currency: trs.CurrencySwiss},
				{name: "2", amount: 728.35, amountCurrency: 795.05, currency: trs.CurrencySwiss},
				{name: "3", amount: -4.46, amountCurrency: 0, currency: trs.CurrencySwiss},
				{name: "4", amount: 0.32, amountCurrency: 0, currency: trs.CurrencySwiss},
				{name: "5", amount: 14.72, amountCurrency: 16.20, currency: trs.CurrencySwiss},
				{name: "6", amount: -737.10, amountCurrency: -811.25, currency: trs.CurrencySwiss},
			}
			moves := h.AccountMove().NewSet(env)
			for _, val := range moveListVals {
				moves = moves.Union(trs.createMove(env, val))
			}
			amlRecs := h.AccountMoveLine().Search(env, q.AccountMoveLine().Move().In(moves).And().Account().Equals(trs.AccountRcv))
			wizard := h.AccountMoveLineReconcile().NewSet(env).WithContext("active_ids", amlRecs.Ids()).Create(
				h.AccountMoveLineReconcile().NewSet(env).WithContext("active_ids", amlRecs.Ids()).DefaultGet())
			wizard.TransRecReconcileFull()
			for _, aml := range amlRecs.Records() {
				So(aml.Reconciled(), ShouldBeTrue)
				So(aml.AmountResidual(), ShouldEqual, 0)
				So(aml.AmountResidualCurrency(), ShouldEqual, 0)
			}

			moveListVals = []moveLineStruct{
				{name: "2", amount: 728.35, amountCurrency: 795.05, currency: trs.CurrencySwiss},
				{name: "3", amount: -4.46, amountCurrency: 0, currency: trs.CurrencyFalse},
				{name: "4", amount: 0.32, amountCurrency: 0, currency: trs.CurrencyFalse},
				{name: "5", amount: 14.72, amountCurrency: 16.20, currency: trs.CurrencySwiss},
				{name: "6", amount: -737.10, amountCurrency: -811.25, currency: trs.CurrencySwiss},
			}
			moves = h.AccountMove().NewSet(env)
			for _, val := range moveListVals {
				moves = moves.Union(trs.createMove(env, val))
			}
			amlRecs = h.AccountMoveLine().Search(env, q.AccountMoveLine().Move().In(moves).And().Account().Equals(trs.AccountRcv))
			wizard2 := h.AccountMoveLineReconcileWriteoff().NewSet(env).WithContext("active_ids", amlRecs.Ids()).Create(
				h.AccountMoveLineReconcileWriteoff().NewSet(env).WithContext("active_ids", amlRecs.Ids()).DefaultGet().
					SetJournal(trs.BankJournalUsd).
					SetWriteoffAcc(trs.AccountRsa))
			wizard2.TransRecReconcile()
			for _, aml := range amlRecs.Records() {
				So(aml.Reconciled(), ShouldBeTrue)
				So(aml.AmountResidual(), ShouldEqual, 0)
				So(aml.AmountResidualCurrency(), ShouldEqual, 0)
			}
		}), ShouldBeNil)
	})
}

func TestReconcileBankStatementWithPaymentAndWriteoff(t *testing.T) {
	Convey("Test reconcile_bank_statement_with_payment_and_writeoff", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			// Use case:
			// Company is in EUR, create a bill for 80 USD and register payment of 80 USD.
			// create a bank statement in USD bank journal with a bank statement line of 85 USD
			// Reconcile bank statement with payment and put the remaining 5 USD in bank fees or another account.

			invoice := trs.createInvoice(env, "out_invoice", 80, trs.CurrencyUsd)
			// register payment on invoice
			payment := h.AccountPayment().Create(env,
				h.AccountPayment().NewData().
					SetPaymentType("inbound").
					SetPaymentMethod(h.AccountPaymentMethod().NewSet(env).GetRecord("account_account_payment_method_manual_in")).
					SetPartnerType("customer").
					SetPartner(trs.PartnerAgrolait).
					SetAmount(80).
					SetCurrency(trs.CurrencyUsd).
					SetPaymentDate(dates.ParseDate("2015-07-15")).
					SetJournal(trs.BankJournalUsd))
			payment.Post()
			paymentMoveLine := h.AccountMoveLine().NewSet(env)
			bankMoveLine := h.AccountMoveLine().NewSet(env)
			for _, l := range payment.MoveLines().Records() {
				if l.Account().Equals(trs.AccountRcv) {
					paymentMoveLine = l
				} else {
					bankMoveLine = l
				}
			}
			invoice.RegisterPayment(paymentMoveLine, h.AccountAccount().NewSet(env), h.AccountJournal().NewSet(env))

			// create bank statement
			bankStmt := h.AccountBankStatement().Create(env,
				h.AccountBankStatement().NewData().
					SetJournal(trs.BankJournalUsd).
					SetDate(dates.ParseDate("2015-07-15")))

			bankStmtLine := h.AccountBankStatementLine().Create(env,
				h.AccountBankStatementLine().NewData().
					SetName("payment").
					SetStatement(bankStmt).
					SetPartner(trs.PartnerAgrolait).
					SetAmount(85).
					SetDate(dates.ParseDate("2015-07-15")))

			// reconcile the statement with invoice and put remaining in another account
			bankStmtLine.ProcessReconciliation(bankMoveLine, nil,
				[]accounttypes.BankStatementAMLStruct{{
					AccountID: trs.DiffIncomeAccount.ID(),
					Debit:     0,
					Credit:    5,
					Name:      "bank fees",
				}})

			// Check that move lines associated to bank_statement are correct
			bankStmtAml := h.AccountMoveLine().Search(env, q.AccountMoveLine().Statement().Equals(bankStmt))
			for _, bsa := range bankStmtAml.Records() {
				for _, move := range bsa.Move().Records() {
					for _, line := range move.Lines().Records() {
						bankStmtAml = bankStmtAml.Union(line)
					}
				}
			}
			So(bankStmtAml.Len(), ShouldEqual, 4)

			accLines := []amlStruct{
				{debit: 3.27, amountCurrency: 5, currency: trs.CurrencyUsd},
				{debit: 52.33, amountCurrency: 80, currency: trs.CurrencyUsd},
			}
			lines := amlMap{
				trs.DiffIncomeAccount.ID(): {credit: 3.27, amountCurrency: -5, currency: trs.CurrencyUsd},
				trs.AccountRcv.ID():        {credit: 52.33, amountCurrency: -80, currency: trs.CurrencyUsd},
			}
			for _, aml := range bankStmtAml.Records() {
				line := lines[aml.Account().ID()]
				if aml.Account().Equals(trs.AccountUsd) {
					// find correct line
					line = accLines[1]
					if nbutils.Round(aml.Debit(), 0.01) == accLines[0].debit {
						line = accLines[0]
					}
				}
				So(nbutils.Round(aml.Debit(), 0.01), ShouldEqual, line.debit)
				So(nbutils.Round(aml.Credit(), 0.01), ShouldEqual, line.credit)
				So(nbutils.Round(aml.AmountCurrency(), 0.01), ShouldEqual, line.amountCurrency)
				So(aml.Currency().Equals(line.currency), ShouldBeTrue)
			}
		}), ShouldBeNil)
	})
}

func TestPartialReconcileCurrencies(t *testing.T) {
	Convey("Test Partial Reconcile Currencies", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)

			//                client Account (payable, rsa)
			//        Debit                      Credit
			// --------------------------------------------------------
			// Pay a : 25/0.5 = 50       |   Inv a : 50/0.5 = 100
			// Pay b: 50/0.75 = 66.66    |   Inv b : 50/0.75 = 66.66
			// Pay c: 25/0.8 = 31.25     |
			//
			// Debit_currency = 100      | Credit currency = 100
			// Debit = 147.91            | Credit = 166.66
			// Balance Debit = 18.75
			// Counterpart Credit goes in Exchange diff

			mainCpny := h.Company().NewSet(env).GetRecord("base_main_company")

			destJournal := h.AccountJournal().Search(env, q.AccountJournal().
				Type().Equals("purchase").And().
				Company().Equals(mainCpny))
			accountExpenses := h.AccountAccount().Search(env, q.AccountAccount().
				UserType().Equals(trs.AccountTypeExpenses))

			data := h.AccountJournal().NewData().
				SetDefaultCreditAccount(trs.AccountRsa).
				SetDefaultDebitAccount(trs.AccountRsa)

			trs.BankJournalEuro.Write(data)
			destJournal.Write(data)

			// Setting up rates for USD (main_company is in EUR)
			h.CurrencyRate().Create(env,
				h.CurrencyRate().NewData().
					SetName(dates.ParseDateTime("2015-07-01 00:00:00")).
					SetRate(0.5).
					SetCurrency(trs.CurrencyUsd).
					SetCompany(mainCpny))
			h.CurrencyRate().Create(env,
				h.CurrencyRate().NewData().
					SetName(dates.ParseDateTime("2015-08-01 00:00:00")).
					SetRate(0.75).
					SetCurrency(trs.CurrencyUsd).
					SetCompany(mainCpny))
			h.CurrencyRate().Create(env,
				h.CurrencyRate().NewData().
					SetName(dates.ParseDateTime("2015-09-01 00:00:00")).
					SetRate(0.8).
					SetCurrency(trs.CurrencyUsd).
					SetCompany(mainCpny))

			// Preparing Invoices (from vendor)
			baseInvoiceData := h.AccountInvoice().NewData().
				SetPartner(trs.PartnerAgrolait).
				SetReferenceType("none").
				SetCurrency(trs.CurrencyUsd).
				SetName("invoice to vendor").
				SetAccount(trs.AccountRsa).
				SetType("in_invoice")
			invoiceA := h.AccountInvoice().Create(env, baseInvoiceData.SetDateInvoice(dates.ParseDate("2015-07-02")))
			invoiceB := h.AccountInvoice().Create(env, baseInvoiceData.SetDateInvoice(dates.ParseDate("2015-08-02")))

			baseInvoiceLine := h.AccountInvoiceLine().NewData().
				SetProduct(trs.Product).
				SetQuantity(1).
				SetPriceUnit(50).
				SetName("product that cost 50").
				SetAccount(accountExpenses)
			h.AccountInvoiceLine().Create(env, baseInvoiceLine.SetInvoice(invoiceA))
			h.AccountInvoiceLine().Create(env, baseInvoiceLine.SetInvoice(invoiceB))

			invoiceA.ActionInvoiceOpen()
			invoiceB.ActionInvoiceOpen()

			// Preparing Payments
			basePaymentData := h.AccountPayment().NewData().
				SetPaymentType("outbound").
				SetCurrency(trs.CurrencyUsd).
				SetJournal(trs.BankJournalEuro).
				SetCompany(mainCpny).
				SetPartner(trs.PartnerAgrolait).
				SetPaymentMethod(h.AccountPaymentMethod().NewSet(env).GetRecord("account_account_payment_method_manual_out")).
				SetDestinationJournal(destJournal).
				SetPartnerType("supplier")

			// One partial for invoice_a (fully assigned to it)
			paymentA := h.AccountPayment().Create(env, basePaymentData.
				SetAmount(25).
				SetPaymentDate(dates.ParseDate("2015-07-02")))
			// One that will complete the payment of a, the rest goes to b
			paymentB := h.AccountPayment().Create(env, basePaymentData.
				SetAmount(50).
				SetPaymentDate(dates.ParseDate("2015-08-02")))
			// The last one will complete the payment of b
			paymentC := h.AccountPayment().Create(env, basePaymentData.
				SetAmount(25).
				SetPaymentDate(dates.ParseDate("2015-09-01")))

			paymentA.Post()
			paymentB.Post()
			paymentC.Post()

			filterFunc := func(r m.AccountMoveLineSet) bool {
				return r.Debit() != 0.0 && r.Account().Equals(destJournal.DefaultDebitAccount())
			}

			debitLineA := paymentA.MoveLines().Filtered(filterFunc)
			debitLineB := paymentB.MoveLines().Filtered(filterFunc)
			debitLineC := paymentC.MoveLines().Filtered(filterFunc)

			invoiceA.AssignOutstandingCredit(debitLineA)
			invoiceA.AssignOutstandingCredit(debitLineB)
			invoiceB.AssignOutstandingCredit(debitLineB)
			invoiceB.AssignOutstandingCredit(debitLineC)

			// Asserting correctness (only in the payable account)
			fullReconcile := h.AccountFullReconcile().NewSet(env)
			for _, inv := range invoiceA.Union(invoiceB).Records() {
				So(inv.Reconciled(), ShouldBeTrue)
				for _, aml := range inv.PaymentMoveLines().Union(inv.Move().Lines()).Records() {
					if !aml.Account().Equals(trs.AccountRsa) {
						continue
					}
					So(aml.AmountResidual(), ShouldEqual, 0)
					So(aml.AmountResidualCurrency(), ShouldEqual, 0)
					So(aml.Reconciled(), ShouldBeTrue)
					if fullReconcile.IsEmpty() {
						fullReconcile = aml.FullReconcile()
					} else {
						So(aml.FullReconcile().Equals(fullReconcile), ShouldBeTrue)
					}
				}
			}

			fullRecMove := fullReconcile.ExchangeMove()
			// Globally check whether the amount is correct
			So(fullRecMove.Amount(), ShouldEqual, 18.75)

			// Checking if the direction of the move is correct
			fullRecPayable := fullRecMove.Lines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.Account().Equals(trs.AccountRsa)
			})
			So(fullRecPayable.Balance(), ShouldEqual, 18.75)
		}), ShouldBeNil)
	})
}

func TestUnreconcile(t *testing.T) {
	Convey("Tests Unreconcile", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			// test fails because trs.PartnerArgolait.PropertyAccountReceivable somehow can not be set with datas on startup.
			// see file demo/1011-Partner_update.csv to see that data
			trs := initTestReconciliationStruct(env)

			// Use case:
			// 2 invoices paid with a single payment. Unreconcile the payment with one invoice, the
			// other invoice should remain reconciled.

			inv1 := trs.createInvoice(env, "out_invoice", 10, trs.CurrencyUsd)
			inv2 := trs.createInvoice(env, "out_invoice", 20, trs.CurrencyUsd)

			payment := h.AccountPayment().Create(env,
				h.AccountPayment().NewData().
					SetPaymentType("inbound").
					SetPaymentMethod(h.AccountPaymentMethod().NewSet(env).GetRecord("account_account_payment_method_manual_in")).
					SetPartnerType("customer").
					SetPartner(trs.PartnerAgrolait).
					SetAmount(100).
					SetCurrency(trs.CurrencyUsd).
					SetJournal(trs.BankJournalUsd))
			payment.Post()
			creditAml := payment.MoveLines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.Credit() != 0.0
			})

			// Check residual before assignation
			So(inv1.Residual(), ShouldAlmostEqual, 10)
			So(inv2.Residual(), ShouldAlmostEqual, 20)

			// Assign credit and residual
			inv1.AssignOutstandingCredit(creditAml)
			inv2.AssignOutstandingCredit(creditAml)
			So(inv1.Residual(), ShouldAlmostEqual, 0)
			So(inv2.Residual(), ShouldAlmostEqual, 0)

			// Unreconcile one invoice at a time and check residual
			creditAml.WithContext("invoice_id", inv1.ID()).RemoveMoveReconcile()
			So(inv1.Residual(), ShouldAlmostEqual, 10)
			So(inv2.Residual(), ShouldAlmostEqual, 0)
			creditAml.WithContext("invoice_id", inv2.ID()).RemoveMoveReconcile()
			So(inv1.Residual(), ShouldAlmostEqual, 10)
			So(inv2.Residual(), ShouldAlmostEqual, 20)
		}), ShouldBeNil)
	})
}

func TestUnreconcileExchange(t *testing.T) {
	Convey("Test Unreconcile Exchange", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)

			// Use case:
			// - Company currency in EUR
			// - Create 2 rates for USD:
			//   1.0 on 2018-01-01
			//   0.5 on 2018-02-01
			// - Create an invoice on 2018-01-02 of 111 USD
			// - Register a payment on 2018-02-02 of 111 USD
			// - Unreconcile the payment

			baseCurrencyRateData := h.CurrencyRate().NewData().
				SetCurrency(trs.CurrencyUsd).
				SetCompany(h.Company().NewSet(env).GetRecord("base_main_company"))
			h.CurrencyRate().Create(env, baseCurrencyRateData.
				SetName(dates.Now().SetMonth(7).SetDay(1)).
				SetRate(1))
			h.CurrencyRate().Create(env, baseCurrencyRateData.
				SetName(dates.Now().SetMonth(8).SetDay(1)).
				SetRate(0.5))
			inv := trs.createInvoice(env, "out_invoice", 111, trs.CurrencyUsd)

			payment := h.AccountPayment().Create(env, h.AccountPayment().NewData().
				SetPaymentType("inbound").
				SetPaymentMethod(h.AccountPaymentMethod().NewSet(env).GetRecord("account_account_payment_method_manual_in")).
				SetPartnerType("customer").
				SetPartner(trs.PartnerAgrolait).
				SetAmount(111).
				SetCurrency(trs.CurrencyUsd).
				SetJournal(trs.BankJournalUsd).
				SetPaymentDate(dates.Today().SetMonth(8).SetDay(1)))
			payment.Post()
			creditAml := payment.MoveLines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.Credit() != 0
			})

			// Check residual before assignation
			So(inv.Residual(), ShouldAlmostEqual, 111)

			// Assign credit, check exchange move and residual
			inv.AssignOutstandingCredit(creditAml)
			mapped := h.AccountFullReconcile().NewSet(env)
			for _, line := range payment.MoveLines().Records() {
				mapped = mapped.Union(line.FullReconcile())
			}
			So(mapped.ExchangeMove().Len(), ShouldEqual, 1)
			So(inv.Residual(), ShouldAlmostEqual, 0)

			// Unreconcile invoice and check residual
			creditAml.WithContext("invoice_id", inv.ID()).RemoveMoveReconcile()
			So(inv.Residual(), ShouldAlmostEqual, 111)
		}), ShouldBeNil)
	})
}

func TestRevertPaymentAndReconcile(t *testing.T) {
	Convey("Test Revert Payment And Reconcile", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)

			payment := h.AccountPayment().Create(env, h.AccountPayment().NewData().
				SetPaymentMethod(trs.InboundPaymentMethod).
				SetPaymentType("inbound").
				SetPartnerType("customer").
				SetPartner(trs.PartnerAgrolait).
				SetJournal(trs.BankJournalUsd).
				SetPaymentDate(dates.ParseDate("2018-06-04")).
				SetAmount(666))
			payment.Post()
			So(payment.MoveLines().Len(), ShouldEqual, 2)

			bankLine := payment.MoveLines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.Account().Equals(trs.BankJournalUsd.DefaultDebitAccount())
			})
			customerLine := payment.MoveLines().Subtract(bankLine)

			So(bankLine.Len(), ShouldEqual, 1)
			So(customerLine.Len(), ShouldEqual, 1)
			So(bankLine.Equals(customerLine), ShouldBeFalse)
			So(bankLine.Move().Equals(customerLine.Move()), ShouldBeTrue)
			move := bankLine.Move()

			// Reversing the payment's move
			reversedMoveList := move.ReverseMove(dates.Date{}.SetYear(2018).SetMonth(06).SetDay(04), h.AccountJournal().NewSet(env))
			So(reversedMoveList.Len(), ShouldEqual, 1)
			reversedMove := reversedMoveList.Records()[0]
			So(reversedMove.Lines().Len(), ShouldEqual, 2)

			// Testing the reconciliation matching between the move lines and their reversed counterparts
			reversedBankLine := reversedMove.Lines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.Account().Equals(trs.BankJournalUsd.DefaultDebitAccount())
			})
			reversedCustomerLine := reversedMove.Lines().Subtract(reversedBankLine)

			So(reversedBankLine.Len(), ShouldEqual, 1)
			So(reversedCustomerLine.Len(), ShouldEqual, 1)
			So(reversedBankLine.Equals(reversedCustomerLine), ShouldBeFalse)
			So(reversedBankLine.Move().Equals(reversedCustomerLine.Move()), ShouldBeTrue)
			So(reversedBankLine.FullReconcile().Equals(bankLine.FullReconcile()), ShouldBeTrue)
			So(reversedCustomerLine.FullReconcile().Equals(customerLine.FullReconcile()), ShouldBeTrue)
		}), ShouldBeNil)
	})
}

func TestAgedReport(t *testing.T) {
	Convey("Test Aged Report", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			agedReport := h.ReportAccountReportAgedpartnerbalance().NewSet(env).WithContext("include_nullified_amount", true)
			reportDateTo := dates.ParseDate("2015-07-15")
			partner := h.Partner().Create(env, h.Partner().NewData().SetName("AgedPartner"))
			currency := h.User().NewSet(env).CurrentUser().Company().Currency()

			invoice := trs.createInvoicePartner(env, "out_invoice", 50, currency, partner)
			journal := h.AccountJournal().Create(env, h.AccountJournal().NewData().
				SetName("Bank").
				SetType("bank").
				SetCode("THE"))
			statement := trs.makePayment(env, invoice, journal, 50, 0, h.Currency().NewSet(env))

			// The report searches on the create_date to dispatch reconciled lines to report periods
			// Also, in this case, there can be only 1 partial_reconcile
			statementPartial := h.AccountPartialReconcile().NewSet(env)
			for _, l := range statement.MoveLines().Records() {
				statementPartial = statementPartial.Union(l.MatchedCredits().Union(l.MatchedDebits()))
			}
			env.Cr().Execute(`UPDATE account_partial_reconcile SET create_date = ? WHERE id IN (?)`,
				reportDateTo, statementPartial.Ids())

			// Case 1: The invoice and payment are reconciled: Nothing should appear
			reportLines, _, amls := agedReport.GetPartnerMoveLines([]string{"receivable"}, reportDateTo, "posted", 30)
			var partnerLines []accounttypes.AgedBalanceReportValues
			for _, l := range reportLines {
				if l.PartnerID == partner.ID() {
					partnerLines = append(partnerLines, l)
				}
			}
			So(partnerLines, ShouldBeEmpty)
			So(amls[partner.ID()], ShouldBeEmpty)

			// Case 2: The invoice and payment are not reconciled: we should have one line on the report
			// and 2 amls
			invoice.Move().Lines().WithContext("invoice_id", invoice.ID()).RemoveMoveReconcile()
			reportLines, _, amls = agedReport.GetPartnerMoveLines([]string{"receivable"}, reportDateTo, "posted", 30)

			partnerLines = []accounttypes.AgedBalanceReportValues{}
			for _, l := range reportLines {
				if l.PartnerID == partner.ID() {
					partnerLines = append(partnerLines, l)
				}
			}
			So(partnerLines, ShouldHaveLength, 1)
			p0 := partnerLines[0]
			So(p0.Trust, ShouldBeTrue)
			So(p0.Values, ShouldEqual, [5]float64{0, 0, 0, 0, 0})
			So(p0.Direction, ShouldEqual, 0)
			So(p0.PartnerID, ShouldEqual, partner.ID())
			So(p0.Name, ShouldEqual, "AgedPartner")
			So(amls[partner.ID()], ShouldHaveLength, 2)

			var positiveLine, negativeLine accounttypes.AgedBalanceReportLine
			for _, l := range amls[partner.ID()] {
				lineRec := h.AccountMoveLine().BrowseOne(env, l.LineID)
				if lineRec.Balance() > 0 {
					positiveLine = l
				}
				if lineRec.Balance() < 0 {
					negativeLine = l
				}
			}
			So(positiveLine.Amount, ShouldEqual, 50)
			So(negativeLine.Amount, ShouldEqual, -50)
		}), ShouldBeNil)
	})
}

func TestRevertPaymentAndReconcileExchange(t *testing.T) {
	Convey("Test Revert Payment And Reconcile Exchange", t, FailureContinues, func() {
		So(models.SimulateInNewEnvironment(security.SuperUserID, func(env models.Environment) {
			trs := initTestReconciliationStruct(env)
			baseCurrencyRateData := h.CurrencyRate().NewData().
				SetCurrency(trs.CurrencyUsd).
				SetCompany(h.Company().NewSet(env).GetRecord("base_main_company"))
			h.CurrencyRate().Create(env, baseCurrencyRateData.
				SetName(dates.ParseDateTime("2015-07-01 00:00:00")).
				SetRate(1))
			h.CurrencyRate().Create(env, baseCurrencyRateData.
				SetName(dates.ParseDateTime("2015-08-01 00:00:00")).
				SetRate(0.5))
			inv := trs.createInvoice(env, "out_invoice", 111, trs.CurrencyUsd)
			payment := h.AccountPayment().Create(env, h.AccountPayment().NewData().
				SetPaymentType("inbound").
				SetPaymentMethod(h.AccountPaymentMethod().NewSet(env).GetRecord("account_account_payment_method_manual_in")).
				SetPartnerType("customer").
				SetPartner(trs.PartnerAgrolait).
				SetAmount(111).
				SetCurrency(trs.CurrencyUsd).
				SetJournal(trs.BankJournalUsd).
				SetPaymentDate(dates.ParseDate("2015-08-01")))
			payment.Post()

			creditAml := payment.MoveLines().Filtered(func(set m.AccountMoveLineSet) bool {
				return set.Credit() != 0
			})
			inv.AssignOutstandingCredit(creditAml)
			So(inv.State(), ShouldEqual, "paid")

			exchangeReconcile := h.AccountFullReconcile().NewSet(env)
			for _, line := range payment.MoveLines().Records() {
				exchangeReconcile = exchangeReconcile.Union(line.FullReconcile())
			}
			exchangeMove := exchangeReconcile.ExchangeMove()
			paymentMove := payment.MoveLines().Records()[0].Move()

			revertedPaymentMove := paymentMove.ReverseMoves(dates.ParseDate("2015-08-01"), h.AccountJournal().NewSet(env))

			// After reversal of payment, the invoice should be open
			So(inv.State(), ShouldEqual, "open")
			exchangeReconcile.ForceLoad("ID")
			So(exchangeReconcile.IsEmpty(), ShouldBeTrue)

			revertedPaymentMove = h.AccountMove().Search(env, q.AccountMove().
				Journal().Equals(exchangeMove.Journal()).And().
				Ref().Contains(exchangeMove.Name())).Limit(1)

			trs.moveRevertTestPair(paymentMove, revertedPaymentMove)
			trs.moveRevertTestPair(exchangeMove, revertedPaymentMove)

		}), ShouldBeNil)
	})
}
