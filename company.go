package account

import (
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/strutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/q"
	"strconv"
	"strings"
)

func init() {

	h.Company().AddFields(map[string]models.FieldDefinition{
		"FiscalyearLastDay": models.IntegerField{
			Default:  models.DefaultValue(31),
			Required: true},
		"FiscalyearLastMonth": models.SelectionField{
			Selection: types.Selection{
				"1":  "January",
				"2":  "February",
				"3":  "March",
				"4":  "April",
				"5":  "May",
				"6":  "June",
				"7":  "July",
				"8":  "August",
				"9":  "September",
				"10": "October",
				"11": "November",
				"12": "December"},
			Default:  models.DefaultValue("12"),
			Required: true},
		"PeriodLockDate": models.DateField{
			String: "Lock Date for Non-Advisers",
			Help: `Only users with the 'Adviser' role can edit accounts prior to and inclusive of this date.
Use it for period locking inside an open fiscal year, for example.`},
		"FiscalyearLockDate": models.DateField{
			String: "Lock Date",
			Help: `No users, including Advisers, can edit accounts prior to and inclusive of this date.
Use it for fiscal year locking for example.`},
		"TransferAccount": models.Many2OneField{
			String:        "Inter-Banks Transfer Account",
			RelationModel: h.AccountAccount(),
			Filter: q.AccountAccount().Reconcile().Equals(true).
				And().Deprecated().Equals(false).
				And().UserTypeFilteredOn(
				q.AccountAccountType().HexyaExternalID().Equals("account.data_account_type_current_assets")),
			Help: "Intermediary account used when moving money from a liquidity account to another"},
		"ExpectsChartOfAccounts": models.BooleanField{
			String:  "Expects a Chart of Accounts",
			Default: models.DefaultValue(true)},
		"ChartTemplate": models.Many2OneField{
			RelationModel: h.AccountChartTemplate(),
			Help:          "The chart template for the company (if any)"},
		"BankAccountCodePrefix": models.CharField{
			String: "Prefix of the bank accounts"},
		"CashAccountCodePrefix": models.CharField{
			String: "Prefix of the cash accounts"},
		"AccountsCodeDigits": models.IntegerField{
			String: "Number of digits in an account code"},
		"TaxCalculationRoundingMethod": models.SelectionField{
			String: "Tax Calculation Rounding Method",
			Selection: types.Selection{
				"round_per_line": "Round per Line",
				"round_globally": "Round Globally"},
			Default: models.DefaultValue("round_per_line"),
			Help: `If you select 'Round per Line' : for each tax the tax amount will first be computed and
rounded for each PO/SO/invoice line and then these rounded amounts will be summed
leading to the total amount for that tax.
If you select 'Round Globally': for each tax the tax amount will be computed for
each PO/SO/invoice line then these amounts will be summed and eventually this
total tax amount will be rounded. If you sell with tax included you should
choose 'Round per line' because you certainly want the sum of your tax-included
line subtotals to be equal to the total amount with taxes.`},
		"CurrencyExchangeJournal": models.Many2OneField{
			String:        "Exchange Gain or Loss Journal",
			RelationModel: h.AccountJournal(),
			Filter:        q.AccountJournal().Type().Equals("general")},
		"IncomeCurrencyExchangeAccount": models.Many2OneField{
			String:        "Gain Exchange Rate Account",
			RelationModel: h.AccountAccount(),
			Related:       "CurrencyExchangeJournal.DefaultCreditAccount",
			Filter: q.AccountAccount().InternalType().Equals("other").
				And().Deprecated().Equals(false).
				And().Company().EqualsEval("id")},
		"ExpenseCurrencyExchangeAccount": models.Many2OneField{
			String:        "Loss Exchange Rate Account",
			RelationModel: h.AccountAccount(),
			Related:       "CurrencyExchangeJournal.DefaultDebitAccount",
			Filter: q.AccountAccount().InternalType().Equals("other").
				And().Deprecated().Equals(false).
				And().Company().EqualsEval("id")},
		"AngloSaxonAccounting": models.BooleanField{
			String: "Use anglo-saxon accounting"},
		"PropertyStockAccountInputCateg": models.Many2OneField{
			String:        "Input Account for Stock Valuation",
			RelationModel: h.AccountAccount()},
		"PropertyStockAccountOutputCateg": models.Many2OneField{
			String:        "Output Account for Stock Valuation",
			RelationModel: h.AccountAccount()},
		"PropertyStockValuationAccount": models.Many2OneField{
			String:        "Account Template for Stock Valuation",
			RelationModel: h.AccountAccount()},
		"BankJournals": models.One2ManyField{
			RelationModel: h.AccountJournal(),
			ReverseFK:     "Company",
			JSON:          "bank_journal_ids",
			Filter:        q.AccountJournal().Type().Equals("bank")},
		"OverdueMsg": models.TextField{
			String:    "Overdue Payments Message",
			Translate: true,
			Default: models.DefaultValue(`Dear Sir/Madam,

Our records indicate that some payments on your account are still due. Please find details below.
If the amount has already been paid, please disregard this notice. Otherwise, please forward us the total amount stated below.
If you have any queries regarding your account, Please contact us.

Thank you in advance for your cooperation.
Best Regards,`)},
	})

	h.Company().Methods().ComputeFiscalyearDates().DeclareMethod(
		`ComputeFiscalyearDates Computes the start and end dates of the fiscalyear where the given 'date' belongs to
			      @param date: a datetime object
			      @returns: a dictionary with date_from and date_to`,
		func(rs h.CompanySet, date dates.Date) (dates.Date, dates.Date) {
			var lastMonth int
			var lastDay int
			var dateTo dates.Date
			var dateFrom dates.Date

			rs = rs.Records()[0]
			lastMonth, _ = strconv.Atoi(rs.FiscalyearLastMonth())
			lastDay = int(rs.FiscalyearLastDay())

			if int(date.Month()) < lastMonth || (int(date.Month()) == lastMonth && date.Day() <= lastDay) {
				date = date.SetMonth(lastMonth).SetDay(lastDay)
			} else {
				if lastMonth == 2 && lastDay == 29 && (date.Year()+1)%4 != 0 {
					date = date.SetMonth(lastMonth).SetDay(28).SetYear(date.Year() + 1)
				} else {
					date = date.SetMonth(lastMonth).SetDay(lastDay).SetYear(date.Year() + 1)
				}
			}

			dateTo = date
			dateFrom = date.AddDate(0, 0, 1)
			if dateFrom.Month() == 2 && dateFrom.Day() == 29 {
				dateFrom = dateFrom.SetDay(28).SetYear(dateFrom.Year() - 1)
			} else {
				dateFrom = dateFrom.SetYear(dateFrom.Year() - 1)
			}
			return dateFrom, dateTo
		})

	h.Company().Methods().GetNewAccountCode().DeclareMethod(
		`GetNewAccountCode`,
		func(rs h.CompanySet, currentCode, oldPrefix, newPrefix string, digits int) string {
			code := strings.Replace(currentCode, oldPrefix, "", 1)
			code = strings.TrimLeft(code, "0")
			code = strutils.RightJustify(code, digits-len(newPrefix), "0")
			return newPrefix + code
		})

	h.Company().Methods().ReflectCodePrefixChange().DeclareMethod(
		`ReflectCodePrefixChange`,
		func(rs h.CompanySet, oldCode, newCode string, digits int) {
			accounts := h.AccountAccount().Search(rs.Env(),
				q.AccountAccount().Code().Contains(oldCode).
					And().InternalType().Equals("liquidity").
					And().Company().Equals(rs)).OrderBy("code asc")

			for _, account := range accounts.Records() {
				if strings.HasPrefix(account.Code(), oldCode) {
					account.SetCode(rs.GetNewAccountCode(account.Code(), oldCode, newCode, digits))
				}
			}
		})

	h.Company().Methods().ReflectCodeDigitsChange().DeclareMethod(
		`ReflectCodeDigitsChange`,
		func(rs h.CompanySet, digits int) {
			accounts := h.AccountAccount().Search(rs.Env(),
				q.AccountAccount().Company().Equals(rs)).OrderBy("code asc")

			for _, account := range accounts.Records() {
				account.SetCode(strutils.LeftJustify(strings.TrimLeft(account.Code(), "0"), digits, "0"))
			}
		})

	h.Company().Methods().ValidateFiscalyearLock().DeclareMethod(
		`ValidateFiscalyearLock`,
		func(rs h.CompanySet, values *h.CompanyData, fieldsToReset ...models.FieldNamer) {
			if values.FiscalyearLockDate().IsZero() {
				return
			}
			if h.AccountMove().Search(rs.Env(),
				q.AccountMove().Company().In(rs).
					And().State().Equals("draft").
					And().Date().LowerOrEqual(values.FiscalyearLockDate()),
			).IsNotEmpty() {
				panic(rs.T(`There are still unposted entries in the period you want to lock. You should either post or delete them.`))
			}
		})

	h.Company().Methods().Write().Extend("",
		func(rs h.CompanySet, data *h.CompanyData) bool {
			// restrict the closing of FY if there are still unposted entries
			rs.ValidateFiscalyearLock(data)

			// Reflect the change on accounts
			for _, company := range rs.Records() {
				digits := data.AccountsCodeDigits()
				if digits == 0 {
					digits = company.AccountsCodeDigits()
				}
				if data.BankAccountCodePrefix() != "" || data.AccountsCodeDigits() != 0 {
					newBankCode := data.BankAccountCodePrefix()
					if newBankCode == "" {
						newBankCode = company.BankAccountCodePrefix()
					}
					company.ReflectCodePrefixChange(company.BankAccountCodePrefix(), newBankCode, int(digits))
				}
				if data.CashAccountCodePrefix() != "" || data.AccountsCodeDigits() != 0 {
					newBankCode := data.CashAccountCodePrefix()
					if newBankCode == "" {
						newBankCode = company.CashAccountCodePrefix()
					}
					company.ReflectCodePrefixChange(company.CashAccountCodePrefix(), newBankCode, int(digits))
				}
				if data.AccountsCodeDigits() != 0 {
					company.ReflectCodeDigitsChange(int(digits))
				}
			}
			return rs.Super().Write(data)
		})

}
