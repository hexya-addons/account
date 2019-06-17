// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hexya-addons/web/webdata"
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func init() {

	h.AccountAccountTemplate().DeclareModel()
	h.AccountAccountTemplate().SetDefaultOrder("code")

	h.AccountAccountTemplate().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			Required: true,
			Index:    true},
		"Currency": models.Many2OneField{
			String:        "Account Currency",
			RelationModel: h.Currency(),
			Help:          "Forces all moves for this account to have this secondary currency."},
		"Code": models.CharField{
			String:   "Code",
			Size:     64,
			Required: true,
			Index:    true},
		"UserType": models.Many2OneField{
			String:        "Type",
			RelationModel: h.AccountAccountType(),
			Required:      true,
			Help: `These types are defined according to your country.
The type contains more information about the account and its specificities.`},
		"Reconcile": models.BooleanField{
			String:  "Allow Invoices & payments Matching",
			Default: models.DefaultValue(false),
			Help:    "Check this option if you want the user to reconcile entries in this account."},
		"Note": models.TextField{},
		"Taxes": models.Many2ManyField{
			String:        "Default Taxes",
			RelationModel: h.AccountTaxTemplate(),
			JSON:          "tax_ids"},
		"Nocreate": models.BooleanField{
			String:  "Optional Create",
			Default: models.DefaultValue(false),
			Help:    "If checked, the new chart of accounts will not contain this by default."},
		"ChartTemplate": models.Many2OneField{
			RelationModel: h.AccountChartTemplate(),
			Help: `This optional field allow you to link an account template to a specific chart template that may
differ from the one its root parent belongs to. This allow you
to define chart templates that extend another and complete it with
few new accounts (You don't need to define the whole structure that
is common to both several times).`},
		"Tags": models.Many2ManyField{
			String:        "Account tag",
			RelationModel: h.AccountAccountTag(),
			JSON:          "tag_ids",
			Help:          "Optional tags you may want to assign for custom reporting"},
	})

	//h.AccountChartTemplate().Fields().DisplayName().SetDepends([]string{"Name", "Code"})

	h.AccountAccountTemplate().Methods().NameGet().Extend("",
		func(rs m.AccountAccountTemplateSet) string {
			name := rs.Name()
			if rs.Code() != "" {
				name = rs.Code() + " " + name
			}
			return name
		})

	h.AccountChartTemplate().DeclareModel()

	h.AccountChartTemplate().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			Required: true},
		"Company": models.Many2OneField{
			RelationModel: h.Company()},
		"Parent": models.Many2OneField{
			String:        "Parent Chart Template",
			RelationModel: h.AccountChartTemplate()},
		"CodeDigits": models.IntegerField{
			String:   "# of Digits",
			Required: true,
			Default:  models.DefaultValue(6),
			Help:     "No. of Digits to use for account code"},
		"Visible": models.BooleanField{
			String:  "Can be Visible?",
			Default: models.DefaultValue(true),
			Help: `Set this to False if you don't want this template to be used actively in the wizard that
generate Chart of Accounts from templates, this is useful when you want to generate
accounts of this template only when loading its child template.`},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency(),
			Required:      true},
		"UseAngloSaxon": models.BooleanField{
			String:  "Use Anglo-Saxon accounting",
			Default: models.DefaultValue(false)},
		"CompleteTaxSet": models.BooleanField{
			String:  "Complete Set of Taxes",
			Default: models.DefaultValue(true),
			Help: `This boolean helps you to choose if you want to propose to the user to encode the sale and
purchase rates or choose from list  of taxes. This last choice assumes that the set of tax
defined on this template is complete`},
		"Accounts": models.One2ManyField{
			String:        "Associated Account Templates",
			RelationModel: h.AccountAccountTemplate(),
			ReverseFK:     "ChartTemplate",
			JSON:          "account_ids"},
		"TaxTemplates": models.One2ManyField{
			String:        "Tax Template List",
			RelationModel: h.AccountTaxTemplate(),
			ReverseFK:     "ChartTemplate",
			JSON:          "tax_template_ids",
			Help:          "List of all the taxes that have to be installed by the wizard"},
		"BankAccountCodePrefix": models.CharField{
			String: "Prefix of the bank accounts"},
		"CashAccountCodePrefix": models.CharField{
			String: "Prefix of the main cash accounts"},
		"TransferAccount": models.Many2OneField{
			RelationModel: h.AccountAccountTemplate(),
			Required:      true,
			Filter: q.AccountAccountTemplate().Reconcile().Equals(true).
				And().UserTypeFilteredOn(
				q.AccountAccountType().HexyaExternalID().Equals("account_data_account_type_current_assets")),
			Help: "Intermediary account used when moving money from a liquidity account to another"},
		"IncomeCurrencyExchangeAccount": models.Many2OneField{
			String:        "Gain Exchange Rate Account",
			RelationModel: h.AccountAccountTemplate()},
		"ExpenseCurrencyExchangeAccount": models.Many2OneField{
			String:        "Loss Exchange Rate Account",
			RelationModel: h.AccountAccountTemplate()},
		"PropertyAccountReceivable": models.Many2OneField{
			String:        "Receivable Account",
			RelationModel: h.AccountAccountTemplate()},
		"PropertyAccountPayable": models.Many2OneField{
			String:        "Payable Account",
			RelationModel: h.AccountAccountTemplate()},
		"PropertyAccountExpenseCateg": models.Many2OneField{
			String:        "Category of Expense Account",
			RelationModel: h.AccountAccountTemplate()},
		"PropertyAccountIncomeCateg": models.Many2OneField{
			String:        "Category of Income Account",
			RelationModel: h.AccountAccountTemplate()},
		"PropertyAccountExpense": models.Many2OneField{
			String:        "Expense Account on Product Template",
			RelationModel: h.AccountAccountTemplate()},
		"PropertyAccountIncome": models.Many2OneField{
			String:        "Income Account on Product Template",
			RelationModel: h.AccountAccountTemplate()},
		"PropertyStockAccountInputCateg": models.Many2OneField{
			String:        "Input Account for Stock Valuation",
			RelationModel: h.AccountAccountTemplate()},
		"PropertyStockAccountOutputCateg": models.Many2OneField{
			String:        "Output Account for Stock Valuation",
			RelationModel: h.AccountAccountTemplate()},
		"PropertyStockValuationAccount": models.Many2OneField{
			String:        "Account Template for Stock Valuation",
			RelationModel: h.AccountAccountTemplate()},
	})

	h.AccountChartTemplate().Methods().TryLoadingForCurrentCompany().DeclareMethod(
		`TryLoadingForCurrentCompany`,
		func(rs m.AccountChartTemplateSet) *actions.Action {
			var company m.CompanySet
			var wizard m.WizardMultiChartsAccountsSet

			rs.EnsureOne()
			company = h.User().NewSet(rs.Env()).CurrentUser().Company()
			// If we don't have any chart of account on this company, install this chart of account
			if company.ChartTemplate().IsEmpty() {
				wizard = h.WizardMultiChartsAccounts().Create(rs.Env(), h.WizardMultiChartsAccounts().NewData().
					SetCompany(company).
					SetChartTemplate(rs).
					SetCodeDigits(rs.CodeDigits()).
					SetTransferAccount(rs.TransferAccount()).
					SetCurrency(rs.Currency()).
					SetBankAccountCodePrefix(rs.BankAccountCodePrefix()).
					SetCashAccountCodePrefix(rs.CashAccountCodePrefix()))
				wizard.OnchangeChartTemplate()
				wizard.Execute()
			}
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

	h.AccountChartTemplate().Methods().OpenSelectTemplateWizard().DeclareMethod(
		`OpenSelectTemplateWizard`,
		func(rs m.AccountChartTemplateSet) *actions.Action {
			actionRec := &actions.Action{
				Type: actions.ActionCloseWindow,
			}
			if rs.Company().ChartTemplate().IsNotEmpty() {
				actionRec = actions.Registry.GetById("account_action_wizard_multi_chart")
			}
			return actionRec
		})

	h.AccountChartTemplate().Methods().GenerateJournals().DeclareMethod(
		`GenerateJournals This method is used for creating journals.

			  :param chart_temp_id: Chart Template Id.
			  :param acc_template_ref: Account templates reference.
			  :param company_id: company_id selected from wizard.multi.charts.accounts.
			  :returns: True`,
		func(rs m.AccountChartTemplateSet, accTemplateRef map[int64]int64, company m.CompanySet,
			journalsData []m.AccountJournalData) bool {

			var journal m.AccountJournalSet
			for _, valsJournal := range rs.PrepareAllJournals(accTemplateRef, company, journalsData) {
				journal = h.AccountJournal().Create(rs.Env(), valsJournal)
				if valsJournal.Type() == "general" && valsJournal.Code() == rs.T(`EXCH`) {
					company.SetCurrencyExchangeJournal(journal)
				}
			}
			return true
		})

	h.AccountChartTemplate().Methods().PrepareAllJournals().DeclareMethod(
		`PrepareAllJournals`,
		func(rs m.AccountChartTemplateSet, accTemplateRef map[int64]int64, company m.CompanySet,
			journalsData []m.AccountJournalData) []m.AccountJournalData {

			getDefaultAccount := func(data m.AccountJournalData, typ string) m.AccountAccountSet {
				var id int64
				// Get the default accounts
				switch {
				case data.Type() == "sale":
					id = accTemplateRef[rs.PropertyAccountIncomeCateg().ID()]
				case data.Type() == "purchase":
					id = accTemplateRef[rs.PropertyAccountExpenseCateg().ID()]
				case data.Type() == "general" && data.Code() == rs.T(`EXCH`):
					if typ == "credit" {
						id = accTemplateRef[rs.IncomeCurrencyExchangeAccount().ID()]
					} else {
						id = accTemplateRef[rs.ExpenseCurrencyExchangeAccount().ID()]
					}
				}
				return h.AccountAccount().BrowseOne(rs.Env(), id)
			}

			var journals []m.AccountJournalData
			var journalsOut []m.AccountJournalData
			var journalOut m.AccountJournalData

			rs.EnsureOne()
			journals = []m.AccountJournalData{
				h.AccountJournal().NewData().
					SetName(rs.T("Customer Invoices")).
					SetType("sale").
					SetCode(rs.T("INV")).
					SetShowOnDashboard(true).
					SetSequence(5),
				h.AccountJournal().NewData().
					SetName(rs.T("Vendor Bills")).
					SetType("purchase").
					SetCode(rs.T("BILL")).
					SetShowOnDashboard(true).
					SetSequence(6),
				h.AccountJournal().NewData().
					SetName(rs.T("Miscellaneous Operations")).
					SetType("general").
					SetCode(rs.T("MISC")).
					SetShowOnDashboard(false).
					SetSequence(7),
				h.AccountJournal().NewData().
					SetName(rs.T("Exchange Difference")).
					SetType("general").
					SetCode(rs.T("EXCH")).
					SetShowOnDashboard(false).
					SetSequence(9),
			}
			journals = append(journals, journalsData...)

			for _, journal := range journals {
				journalOut = h.AccountJournal().NewData().
					SetType(journal.Type()).
					SetName(journal.Name()).
					SetCode(journal.Code()).
					SetCompany(company).
					SetShowOnDashboard(journal.ShowOnDashboard()).
					SetSequence(journal.Sequence()).
					SetDefaultCreditAccount(getDefaultAccount(journal, "credit")).
					SetDefaultDebitAccount(getDefaultAccount(journal, "debit"))

				journalsOut = append(journalsOut, journalOut)
			}
			return journalsOut
		})

	h.AccountChartTemplate().Methods().GenerateProperties().DeclareMethod(
		`GenerateProperties
			  This method used for creating properties.

			  :param self: chart templates for which we need to create properties
			  :param acc_template_ref: Mapping between ids of account templates and real accounts created from them
			  :param company_id: company_id selected from wizard.multi.charts.accounts.
			  :returns: True`,
		func(rs m.AccountChartTemplateSet, accTemplateRef map[int64]int64, company m.CompanySet) bool {

			// tovalid missing self.env['ir.property']
			// tovalid missing self.env['ir.model.fields']

			/*def generate_properties(self, acc_template_ref, company):
			  PropertyObj = self.env['ir.property']
			  todo_list = [
			      ('property_account_receivable_id', 'res.partner', 'account.account'),
			      ('property_account_payable_id', 'res.partner', 'account.account'),
			      ('property_account_expense_categ_id', 'product.category', 'account.account'),
			      ('property_account_income_categ_id', 'product.category', 'account.account'),
			      ('property_account_expense_id', 'product.template', 'account.account'),
			      ('property_account_income_id', 'product.template', 'account.account'),
			  ]
			  for record in todo_list:
			      account = getattr(self, record[0])
			      value = account and 'account.account,' + str(acc_template_ref[account.id]) or False
			      if value:
			          field = self.env['ir.model.fields'].search([('name', '=', record[0]), ('model', '=', record[1]), ('relation', '=', record[2])], limit=1)
			          vals = {
			              'name': record[0],
			              'company_id': company.id,
			              'fields_id': field.id,
			              'value': value,
			          }
			          properties = PropertyObj.search([('name', '=', record[0]), ('company_id', '=', company.id)])
			          if properties:
			              #the property exist: modify it
			              properties.write(vals)
			          else:
			              #create the property
			              PropertyObj.create(vals)
			  stock_properties = [
			      'property_stock_account_input_categ_id',
			      'property_stock_account_output_categ_id',
			      'property_stock_valuation_account_id',
			  ]
			  for stock_property in stock_properties:
			      account = getattr(self, stock_property)
			      value = account and acc_template_ref[account.id] or False
			      if value:
			          company.write({stock_property: value})
			  return True

			*/
			return true
		})

	h.AccountChartTemplate().Methods().InstallTemplate().DeclareMethod(
		`InstallTemplate
				  Recursively load the template objects and create the real objects from them.

			      :param company: company the wizard is running for
			      :param code_digits: number of digits the accounts code should have in the COA
			      :param transfer_account_id: reference to the account template that will be used as intermediary account for transfers between 2 liquidity accounts
			      :param obj_wizard: the current wizard for generating the COA from the templates
			      :param acc_ref: Mapping between ids of account templates and real accounts created from them
			      :param taxes_ref: Mapping between ids of tax templates and real taxes created from them
			      :returns: tuple with a dictionary containing
			          * the mapping between the account template ids and the ids of the real accounts that have been generated
			            from them, as first item,
			          * a similar dictionary for mapping the tax templates and taxes, as second item,
			      :rtype: tuple(dict, dict, dict)`,
		func(rs m.AccountChartTemplateSet, company m.CompanySet, codeDigits int64, transferAccount m.AccountAccountTemplateSet,
			objWizard m.WizardMultiChartsAccountsSet, accRef, taxesRef map[int64]int64) (map[int64]int64, map[int64]int64) {

			rs.EnsureOne()

			if rs.Parent().IsNotEmpty() {
				tmp1, tmp2 := rs.Parent().InstallTemplate(company, codeDigits, transferAccount, h.WizardMultiChartsAccounts().NewSet(rs.Env()), accRef, taxesRef)
				for key, val := range tmp1 {
					accRef[key] = val
				}
				for key, val := range tmp2 {
					taxesRef[key] = val
				}
			}
			tmp1, tmp2 := rs.LoadTemplate(company, codeDigits, transferAccount, accRef, taxesRef)
			for key, val := range tmp1 {
				accRef[key] = val
			}
			for key, val := range tmp2 {
				taxesRef[key] = val
			}
			return accRef, taxesRef
		})

	h.AccountChartTemplate().Methods().LoadTemplate().DeclareMethod(
		`LoadTemplate Generate all the objects from the templates

			      :param company: company the wizard is running for
			      :param code_digits: number of digits the accounts code should have in the COA
			      :param transfer_account_id: reference to the account template that will be used as intermediary account for transfers between 2 liquidity accounts
			      :param acc_ref: Mapping between ids of account templates and real accounts created from them
			      :param taxes_ref: Mapping between ids of tax templates and real taxes created from them
			      :returns: tuple with a dictionary containing
			          * the mapping between the account template ids and the ids of the real accounts that have been generated
			            from them, as first item,
			          * a similar dictionary for mapping the tax templates and taxes, as second item,
			      :rtype: tuple(dict, dict, dict)`,
		func(rs m.AccountChartTemplateSet, company m.CompanySet, codeDigits int64, transferAccount m.AccountAccountTemplateSet,
			accountRef, taxesRef map[int64]int64) (map[int64]int64, map[int64]int64) {

			var data m.AccountTaxData
			var taxTemplateToTax map[int64]int64
			var accountTemplateRef map[int64]int64
			var AccountDict map[int64]struct {
				AccountID       int64
				RefundAccountID int64
			}

			rs.EnsureOne()
			if codeDigits == 0 {
				codeDigits = rs.CodeDigits()
			}
			if transferAccount.IsEmpty() {
				transferAccount = rs.TransferAccount()
			}

			// Generate taxes from templates.
			for _, tax := range rs.TaxTemplates().Records() {
				println("DEBB", tax.Name(), tax.HexyaExternalID(), tax.ChildrenTaxes().Ids(), "DEBB")
			}
			taxTemplateToTax, AccountDict = rs.TaxTemplates().GenerateTax(company)
			for key, val := range taxTemplateToTax {
				taxesRef[key] = val
			}

			// Generating Accounts from templates.
			accountTemplateRef = rs.GenerateAccount(taxesRef, accountRef, int(codeDigits), company)
			for key, val := range accountTemplateRef {
				accountRef[key] = val
			}

			// writing account values after creation of accounts
			company.SetTransferAccount(h.AccountAccount().BrowseOne(rs.Env(), accountTemplateRef[transferAccount.ID()]))
			for id, values := range AccountDict {
				data = h.AccountTax().NewData()
				if values.RefundAccountID != 0 {
					data.SetRefundAccount(h.AccountAccount().BrowseOne(rs.Env(), values.RefundAccountID))
				}
				if values.AccountID != 0 {
					data.SetAccount(h.AccountAccount().BrowseOne(rs.Env(), values.AccountID))
				}
				h.AccountTax().BrowseOne(rs.Env(), id).Write(data)
			}

			// Create Journals - Only done for root chart template
			if rs.Parent().IsEmpty() {
				rs.GenerateJournals(accountRef, company, nil)
			}

			// generate properties function
			rs.GenerateProperties(accountRef, company)

			// Generate Fiscal Position , Fiscal Position Accounts and Fiscal Position Taxes from templates
			rs.GenerateFiscalPosition(taxesRef, accountRef, company)

			// Generate account operation template templates
			rs.GenerateAccountReconcileModel(taxesRef, accountRef, company)

			return accountRef, taxesRef
		})

	h.AccountChartTemplate().Methods().GetAccountVals().DeclareMethod(
		`GetAccountVals This method generates a dictionary of all the values for the account that will be created.`,
		func(rs m.AccountChartTemplateSet, company m.CompanySet, accountTemplate m.AccountAccountTemplateSet,
			codeAcc string, taxTemplateRef map[int64]int64) m.AccountAccountData {

			var taxIds []int64
			var data m.AccountAccountData

			rs.EnsureOne()

			for _, tax := range accountTemplate.Taxes().Records() {
				taxIds = append(taxIds, taxTemplateRef[tax.ID()])
			}

			data = h.AccountAccount().NewData().
				SetName(accountTemplate.Name()).
				SetCode(accountTemplate.Code()).
				SetReconcile(accountTemplate.Reconcile()).
				SetNote(accountTemplate.Note()).
				SetTags(accountTemplate.Tags()).
				SetTaxes(h.AccountTax().Browse(rs.Env(), taxIds)).
				SetCompany(company)

			if accountTemplate.Currency().IsNotEmpty() {
				data.SetCurrency(accountTemplate.Currency())
			}
			if accountTemplate.UserType().IsNotEmpty() {
				data.SetUserType(accountTemplate.UserType())
			}

			return data
		})

	h.AccountChartTemplate().Methods().GenerateAccount().DeclareMethod(
		`GenerateAccount This method for generating accounts from templates.

			      :param tax_template_ref: Taxes templates reference for write taxes_id in account_account.
			      :param acc_template_ref: dictionary with the mappping between the account templates and the real accounts.
			      :param code_digits: number of digits got from wizard.multi.charts.accounts, this is use for account code.
			      :param company_id: company_id selected from wizard.multi.charts.accounts.
			      :returns: return acc_template_ref for reference purpose.
			      :rtype: dict`,
		func(rs m.AccountChartTemplateSet, taxTemplateRef, accTemplateRef map[int64]int64, codeDigits int,
			company m.CompanySet) map[int64]int64 {

			var accTemplate m.AccountAccountTemplateSet
			var query q.AccountAccountTemplateCondition
			var code string
			var data m.AccountAccountData
			var newAccount m.AccountAccountSet

			rs.EnsureOne()
			query = q.AccountAccountTemplate().
				Nocreate().NotEquals(true).
				And().ChartTemplate().Equals(rs)
			accTemplate = h.AccountAccountTemplate().Search(rs.Env(), query).OrderBy("id")

			for _, accountTemplate := range accTemplate.Records() {
				code = accountTemplate.Code()
				if len(code) > 0 && len(code) < codeDigits {
					code = code + strings.Repeat("0", codeDigits-len(code))
				}
				data = rs.GetAccountVals(company, accountTemplate, code, taxTemplateRef)
				newAccount = h.AccountAccount().Create(rs.Env(), data.SetHexyaExternalID(fmt.Sprintf("%d_%s", company.ID(), accTemplate.HexyaExternalID())))
				accTemplateRef[accountTemplate.ID()] = newAccount.ID()
			}
			return accTemplateRef
		})

	h.AccountChartTemplate().Methods().PrepareReconcileModelVals().DeclareMethod(
		`PrepareReconcileModelVals This method generates a dictionary of all the values for the account.reconcile.model that will be created.`,
		func(rs m.AccountChartTemplateSet, company m.CompanySet, accountReconcileModel m.AccountReconcileModelTemplateSet,
			taxTemplateRef, accTemplateRef map[int64]int64) m.AccountReconcileModelData {

			rs.EnsureOne()
			data := h.AccountReconcileModel().NewData().
				SetName(accountReconcileModel.Name()).
				SetSequence(accountReconcileModel.Sequence()).
				SetHasSecondLine(accountReconcileModel.HasSecondLine()).
				SetCompany(company).
				SetAccount(h.AccountAccount().BrowseOne(rs.Env(), accTemplateRef[accountReconcileModel.Account().ID()])).
				SetLabel(accountReconcileModel.Label()).
				SetAmountType(accountReconcileModel.AmountType()).
				SetAmount(accountReconcileModel.Amount()).
				SetSecondLabel(accountReconcileModel.SecondLabel()).
				SetSecondAmountType(accountReconcileModel.SecondAmountType()).
				SetSecondAmount(accountReconcileModel.SecondAmount())

			if val := accountReconcileModel.Tax(); val.IsNotEmpty() {
				data.SetTax(h.AccountTax().BrowseOne(rs.Env(), taxTemplateRef[val.ID()]))
			}
			if val := accountReconcileModel.SecondAccount(); val.IsNotEmpty() {
				data.SetSecondAccount(h.AccountAccount().BrowseOne(rs.Env(), accTemplateRef[val.ID()]))
			}
			if val := accountReconcileModel.SecondTax(); val.IsNotEmpty() {
				data.SetSecondTax(h.AccountTax().BrowseOne(rs.Env(), taxTemplateRef[val.ID()]))
			}

			return data
		})

	h.AccountChartTemplate().Methods().GenerateAccountReconcileModel().DeclareMethod(
		`GenerateAccountReconcileModel This method for generating accounts from templates.

			      :param tax_template_ref: Taxes templates reference for write taxes_id in account_account.
			      :param acc_template_ref: dictionary with the mappping between the account templates and the real accounts.
			      :param company_id: company_id selected from wizard.multi.charts.accounts.
			      :returns: return new_account_reconcile_model for reference purpose.
			      :rtype: dict`,
		func(rs m.AccountChartTemplateSet, taxTemplateRef, accTemplateRef map[int64]int64, company m.CompanySet) bool {

			var accountReconcileModels m.AccountReconcileModelTemplateSet
			var vals m.AccountReconcileModelData

			rs.EnsureOne()
			accountReconcileModels = h.AccountReconcileModelTemplate().Search(rs.Env(),
				q.AccountReconcileModelTemplate().AccountFilteredOn(q.AccountAccountTemplate().ChartTemplate().Equals(rs)))

			for _, ARModel := range accountReconcileModels.Records() {
				vals = rs.PrepareReconcileModelVals(company, accountReconcileModels, accTemplateRef, taxTemplateRef)
				h.AccountReconcileModel().Create(rs.Env(), vals.SetHexyaExternalID(fmt.Sprintf("%d_%s", company.ID(), ARModel.HexyaExternalID())))
			}
			return true
		})

	h.AccountChartTemplate().Methods().GenerateFiscalPosition().DeclareMethod(
		`GenerateFiscalPosition This method generate Fiscal Position, Fiscal Position Accounts and Fiscal Position Taxes from templates.

			      :param chart_temp_id: Chart Template Id.
			      :param taxes_ids: Taxes templates reference for generating account.fiscal.position.tax.
			      :param acc_template_ref: Account templates reference for generating account.fiscal.position.account.
			      :param company_id: company_id selected from wizard.multi.charts.accounts.
			      :returns: True`,
		func(rs m.AccountChartTemplateSet, taxTemplateRef, accTemplateRef map[int64]int64, company m.CompanySet) bool {
			var positions m.AccountFiscalPositionTemplateSet
			var newFp m.AccountFiscalPositionSet
			var taxData m.AccountFiscalPositionTaxData
			var accountData m.AccountFiscalPositionAccountData

			rs.EnsureOne()
			positions = h.AccountFiscalPositionTemplate().Search(rs.Env(),
				q.AccountFiscalPositionTemplate().ChartTemplate().Equals(rs))
			for _, position := range positions.Records() {
				newFp = h.AccountFiscalPosition().Create(rs.Env(), h.AccountFiscalPosition().NewData().
					SetCompany(company).
					SetName(position.Name()).
					SetNote(position.Note()).
					SetHexyaExternalID(fmt.Sprintf("%d_%s", company.ID(), position.HexyaExternalID())))
				for _, tax := range position.Taxes().Records() {
					taxData = h.AccountFiscalPositionTax().NewData().
						SetTaxSrc(h.AccountTax().BrowseOne(rs.Env(), taxTemplateRef[tax.TaxSrc().ID()])).
						SetPosition(newFp)
					if tax.TaxDest().IsNotEmpty() {
						taxData.SetTaxDest(h.AccountTax().BrowseOne(rs.Env(), taxTemplateRef[tax.TaxDest().ID()]))
					}
					h.AccountFiscalPositionTax().Create(rs.Env(), taxData.SetHexyaExternalID(fmt.Sprintf("%d_%s", company.ID(), tax.HexyaExternalID())))
				}
				for _, acc := range position.Accounts().Records() {
					accountData = h.AccountFiscalPositionAccount().NewData().
						SetAccountSrc(h.AccountAccount().BrowseOne(rs.Env(), accTemplateRef[acc.AccountSrc().ID()])).
						SetAccountDest(h.AccountAccount().BrowseOne(rs.Env(), accTemplateRef[acc.AccountDest().ID()])).
						SetPosition(newFp)
					h.AccountFiscalPositionAccount().Create(rs.Env(), accountData.SetHexyaExternalID(fmt.Sprintf("%d_%s", company.ID(), acc.HexyaExternalID())))
				}
			}
			return true
		})

	h.AccountTaxTemplate().DeclareModel()

	h.AccountTaxTemplate().SetDefaultOrder("ID")

	h.AccountTaxTemplate().AddFields(map[string]models.FieldDefinition{
		"ChartTemplate": models.Many2OneField{
			RelationModel: h.AccountChartTemplate(),
			Required:      true},
		"Name": models.CharField{
			String:   "Tax Name",
			Required: true},
		"TypeTaxUse": models.SelectionField{
			String: "Tax Scope",
			Selection: types.Selection{
				"sale":     "Sales",
				"purchase": "Purchases",
				"none":     "None"},
			Required: true,
			Default:  models.DefaultValue("sale"),
			Help: `Determines where the tax is selectable.
Note : 'None' means a tax can't be used by itself however it can still be used in a group.`},
		"AmountType": models.SelectionField{
			String: "Tax Computation",
			Selection: types.Selection{
				"group":    "Group of Taxes",
				"fixed":    "Fixed",
				"percent":  "Percentage of Price",
				"division": "Percentage of Price Tax Included"},
			Default:  models.DefaultValue("percent"),
			Required: true},
		"Active": models.BooleanField{
			Default: models.DefaultValue(true),
			Help:    "Set active to false to hide the tax without removing it."},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Required:      true,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser().Company()
			}},
		"ChildrenTaxes": models.Many2ManyField{
			RelationModel: h.AccountTaxTemplate(),
			JSON:          "children_tax_ids",
			M2MTheirField: "ChildTax",
			M2MOurField:   "ParentTax"},
		"Sequence": models.IntegerField{
			Required: true,
			Default:  models.DefaultValue(1),
			Help:     "The sequence field is used to define order in which the tax lines are applied."},
		"Amount": models.FloatField{
			Required: true,
			Digits:   nbutils.Digits{Precision: 16, Scale: 4}},
		"Account": models.Many2OneField{
			String:        "Tax Account",
			RelationModel: h.AccountAccountTemplate(),
			OnDelete:      models.Restrict,
			Help:          "Account that will be set on invoice tax lines for invoices. Leave empty to use the expense account."},
		"RefundAccount": models.Many2OneField{
			String:        "Tax Account on Refunds",
			RelationModel: h.AccountAccountTemplate(),
			OnDelete:      models.Restrict,
			Help:          "Account that will be set on invoice tax lines for refunds. Leave empty to use the expense account."},
		"Description": models.CharField{
			String: "Display on Invoices"},
		"PriceInclude": models.BooleanField{
			String:  "Included in Price",
			Default: models.DefaultValue(false),
			Help:    "Check this if the price you use on the product and invoices includes this tax."},
		"IncludeBaseAmount": models.BooleanField{
			String:  "Affect Subsequent Taxes",
			Default: models.DefaultValue(false),
			Help:    "If set, taxes which are computed after this one will be computed based on the price tax included."},
		"Analytic": models.BooleanField{
			String: "Analytic Cost",
			Help: `If set, the amount computed by this tax will be assigned to
the same analytic account as the invoice line (if any)`},
		"Tags": models.Many2ManyField{
			String:        "Account tag",
			RelationModel: h.AccountAccountTag(),
			JSON:          "tag_ids",
			Help:          "Optional tags you may want to assign for custom reporting"},
		"TaxGroup": models.Many2OneField{
			RelationModel: h.AccountTaxGroup()},
		"TaxAdjustment": models.BooleanField{
			Default: models.DefaultValue(false)},
	})

	h.AccountTaxTemplate().AddSQLConstraint("name_company_uniq",
		"unique(name, company_id, type_tax_use)",
		"Tax names must be unique !")

	h.AccountTaxTemplate().Methods().NameGet().Extend("",
		func(rs m.AccountTaxTemplateSet) string {
			var name string

			name = rs.Description()
			if name == "" {
				name = rs.Name()
			}
			return name
		})

	h.AccountTaxTemplate().Methods().GetTaxVals().DeclareMethod(
		`GetTaxVals This method generates a dictionary of all the values for the tax that will be created.`,
		func(rs m.AccountTaxTemplateSet, company m.CompanySet) m.AccountTaxData {

			rs.EnsureOne()
			data := h.AccountTax().NewData().
				SetName(rs.Name()).
				SetTypeTaxUse(rs.TypeTaxUse()).
				SetAmountType(rs.AmountType()).
				SetActive(rs.Active()).
				SetCompany(company).
				SetSequence(int(rs.Sequence())).
				SetAmount(rs.Amount()).
				SetDescription(rs.Description()).
				SetPriceInclude(rs.PriceInclude()).
				SetIncludeBaseAmount(rs.IncludeBaseAmount()).
				SetAnalytic(rs.Analytic()).
				SetTags(rs.Tags()).
				SetTaxAdjustment(rs.TaxAdjustment())
			if rs.TaxGroup().IsNotEmpty() {
				data.SetTaxGroup(rs.TaxGroup())
			}
			return data
		})

	h.AccountTaxTemplate().Methods().GenerateTax().DeclareMethod(
		`GenerateTax
				  This method generate taxes from templates.

			      :param company: the company for which the taxes should be created from templates in self
			      :returns: {
			          'tax_template_to_tax': mapping between tax template and the newly generated taxes corresponding,
			          'account_dict': dictionary containing a to-do list with all the accounts to assign on new taxes
			      }`,
		func(rs m.AccountTaxTemplateSet, company m.CompanySet) (map[int64]int64, map[int64]struct {
			AccountID       int64
			RefundAccountID int64
		}) {
			//for _, tax := range rs.Records() {
			//	fmt.Println("DEBB", tax.ID(), tax.Name(), tax.HexyaExternalID(), tax.ChildrenTaxes().Ids(), "DEBB")
			//}
			taxTemplateToTax := make(map[int64]int64)
			todoDict := make(map[int64]struct {
				AccountID       int64
				RefundAccountID int64
			})
			for _, tax := range rs.Records() {
				// Compute children tax ids
				childrenIds := []int64{}
				for _, childTax := range tax.ChildrenTaxes().Records() {
					println(childTax.ID())
					if val, ok := taxTemplateToTax[childTax.ID()]; ok && val != 0 {
						childrenIds = append(childrenIds, val)
					}
				}
				taxData := tax.GetTaxVals(company).SetChildrenTaxes(h.AccountTax().Browse(rs.Env(), childrenIds))
				newTax := h.AccountTax().Create(rs.Env(), taxData.SetHexyaExternalID(fmt.Sprintf("%d_%s", company.ID(), tax.HexyaExternalID())))
				taxTemplateToTax[tax.ID()] = newTax.ID()
				// Since the accounts have not been created yet, we have to wait before filling these fields
				todoDict[newTax.ID()] = struct {
					AccountID       int64
					RefundAccountID int64
				}{tax.Account().ID(), tax.RefundAccount().ID()}
			}

			return taxTemplateToTax, todoDict
		})

	h.AccountFiscalPositionTemplate().DeclareModel()

	h.AccountFiscalPositionTemplate().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:   "Fiscal Position Template",
			Required: true},
		"ChartTemplate": models.Many2OneField{
			RelationModel: h.AccountChartTemplate(),
			Required:      true},
		"Accounts": models.One2ManyField{
			String:        "Account Mapping",
			RelationModel: h.AccountFiscalPositionAccountTemplate(),
			ReverseFK:     "Position",
			JSON:          "account_ids"},
		"Taxes": models.One2ManyField{
			String:        "Tax Mapping",
			RelationModel: h.AccountFiscalPositionTaxTemplate(),
			ReverseFK:     "Position",
			JSON:          "tax_ids"},
		"Note": models.TextField{
			String: "Notes"},
	})

	h.AccountFiscalPositionTaxTemplate().DeclareModel()

	h.AccountFiscalPositionTaxTemplate().AddFields(map[string]models.FieldDefinition{
		"Position": models.Many2OneField{
			String:        "Fiscal Position",
			RelationModel: h.AccountFiscalPositionTemplate(),
			Required:      true,
			OnDelete:      models.Cascade},
		"TaxSrc": models.Many2OneField{
			String:        "Tax Source",
			RelationModel: h.AccountTaxTemplate(),
			Required:      true},
		"TaxDest": models.Many2OneField{
			String:        "Replacement Tax",
			RelationModel: h.AccountTaxTemplate()},
	})

	h.AccountFiscalPositionTaxTemplate().Methods().NameGet().Extend("",
		func(rs m.AccountFiscalPositionTaxTemplateSet) string {
			return rs.Position().NameGet()
		})

	h.AccountFiscalPositionAccountTemplate().DeclareModel()

	h.AccountFiscalPositionAccountTemplate().AddFields(map[string]models.FieldDefinition{
		"Position": models.Many2OneField{
			String:        "Fiscal Mapping",
			RelationModel: h.AccountFiscalPositionTemplate(),
			Required:      true,
			OnDelete:      models.Cascade},
		"AccountSrc": models.Many2OneField{
			String:        "Account Source",
			RelationModel: h.AccountAccountTemplate(),
			Required:      true},
		"AccountDest": models.Many2OneField{
			String:        "Account Destination",
			RelationModel: h.AccountAccountTemplate(),
			Required:      true},
	})

	h.AccountFiscalPositionAccountTemplate().Methods().NameGet().Extend("",
		func(rs m.AccountFiscalPositionAccountTemplateSet) string {
			return rs.Position().NameGet()
		})

	h.WizardMultiChartsAccounts().DeclareTransientModel()
	//h.WizardMultiChartsAccounts().InheritModel(ResConfig)

	h.WizardMultiChartsAccounts().AddFields(map[string]models.FieldDefinition{
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Required:      true},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency(),
			Help:          "Currency as per company's country.",
			Required:      true},
		"OnlyOneChartTemplate": models.BooleanField{
			String: "Only One Chart Template Available"},
		"ChartTemplate": models.Many2OneField{
			String:        "Chart Template",
			RelationModel: h.AccountChartTemplate(),
			Required:      true,
			OnChange:      h.WizardMultiChartsAccounts().Methods().OnchangeChartTemplate()},
		"BankAccounts": models.One2ManyField{
			String:        "Cash and Banks",
			RelationModel: h.AccountBankAccountsWizard(),
			ReverseFK:     "BankAccount",
			JSON:          "bank_account_ids",
			Required:      true},
		"BankAccountCodePrefix": models.CharField{
			String: "Bank Accounts Prefix"},
		"CashAccountCodePrefix": models.CharField{
			String: "Cash Accounts Prefix"},
		"CodeDigits": models.IntegerField{
			String:   "# of Digits",
			Required: true,
			Help:     "No. of Digits to use for account code"},
		"SaleTax": models.Many2OneField{
			String:        "Default Sales Tax",
			RelationModel: h.AccountTaxTemplate()},
		"PurchaseTax": models.Many2OneField{
			String:        "Default Purchase Tax",
			RelationModel: h.AccountTaxTemplate()},
		"SaleTaxRate": models.FloatField{
			String:   "Sales Tax(%)",
			OnChange: h.WizardMultiChartsAccounts().Methods().OnchangeTaxRate()},
		"UseAngloSaxon": models.BooleanField{
			String:  "Use Anglo-Saxon Accounting",
			Related: "ChartTemplate.UseAngloSaxon"},
		"TransferAccount": models.Many2OneField{
			RelationModel: h.AccountAccountTemplate(),
			Required:      true,
			Filter: q.AccountAccountTemplate().Reconcile().Equals(true).
				And().UserTypeFilteredOn(
				q.AccountAccountType().HexyaExternalID().Equals("account_data_account_type_current_assets")),
			Help: "Intermediary account used when moving money from a liquidity account to another"},
		"PurchaseTaxRate": models.FloatField{
			String: "Purchase Tax(%)"},
		"CompleteTaxSet": models.BooleanField{
			String: "Complete Set of Taxes",
			Help: `This boolean helps you to choose if you want to propose to the user to encode the sales and
purchase rates or use the usual m2o fields. This last choice assumes that the
set of tax defined for the chosen template is complete`},
	})

	h.WizardMultiChartsAccounts().Methods().GetChartParents().DeclareMethod(
		`GetChartParents
                  Returns the IDs of all ancestor charts, including the chart itself.
			      (inverse of child_of operator)

			      :param browse_record chart_template: the account.chart.template record
			      :return: the IDS of all ancestor charts, including the chart itself.`,
		func(rs m.WizardMultiChartsAccountsSet, chartTemplate m.AccountChartTemplateSet) m.AccountChartTemplateSet {
			var result m.AccountChartTemplateSet

			result = chartTemplate
			for chartTemplate.Parent().IsNotEmpty() {
				chartTemplate = chartTemplate.Parent()
				result = result.Union(chartTemplate)
			}

			return result
		})

	h.WizardMultiChartsAccounts().Methods().OnchangeTaxRate().DeclareMethod(
		`OnchangeTaxRate`,
		func(rs m.WizardMultiChartsAccountsSet) m.WizardMultiChartsAccountsData {
			data := h.WizardMultiChartsAccounts().NewData()
			if val := rs.SaleTaxRate(); val != 0.0 {
				data.SetPurchaseTaxRate(val)
			}
			return data
		})

	h.WizardMultiChartsAccounts().Methods().OnchangeChartTemplate().DeclareMethod(
		`OnchangeChartTemplateId`,
		func(rs m.WizardMultiChartsAccountsSet) m.WizardMultiChartsAccountsData {
			var data m.WizardMultiChartsAccountsData
			var currency m.CurrencySet
			var charts m.AccountChartTemplateSet
			var saleTax m.AccountTaxTemplateSet
			var purchaseTax m.AccountTaxTemplateSet
			var baseTaxCond q.AccountTaxTemplateCondition
			var saleTaxCond q.AccountTaxTemplateCondition
			var purchaseTaxCond q.AccountTaxTemplateCondition

			data = h.WizardMultiChartsAccounts().NewData()
			if rs.ChartTemplate().IsEmpty() {
				return data
			}

			currency = h.Currency().Coalesce(rs.ChartTemplate().Currency(), h.User().NewSet(rs.Env()).CurrentUser().Company().Currency())
			data.SetCompleteTaxSet(rs.ChartTemplate().CompleteTaxSet()).
				SetCurrency(currency)
			if rs.ChartTemplate().CompleteTaxSet() {
				//default tax is given by the lowest sequence. For same sequence we will take the latest created as it will be the case for tax created while installing the generic chart of account
				charts = rs.GetChartParents(rs.ChartTemplate())
				// FIXME
				fmt.Println("chart", charts)
				/* base_tax_domain = [('chart_template_id', 'parent_of', chart_ids)] tovalid missing "parent_of" Operator */
				saleTaxCond = baseTaxCond.And().TypeTaxUse().Equals("sale")
				purchaseTaxCond = baseTaxCond.And().TypeTaxUse().Equals("purchase")
				saleTax = h.AccountTaxTemplate().Search(rs.Env(), saleTaxCond)
				purchaseTax = h.AccountTaxTemplate().Search(rs.Env(), purchaseTaxCond)
				data.SetSaleTax(saleTax).
					SetPurchaseTax(purchaseTax)
				/*    res.setdefault('domain', {})     tovalid missing domain in data (NYI)
				      res['domain']['sale_tax_id'] = repr(sale_tax_domain)
				      res['domain']['purchase_tax_id'] = repr(purchase_tax_domain)*/
			}

			if val := rs.ChartTemplate().TransferAccount(); val.IsNotEmpty() {
				data.SetTransferAccount(val)
			}
			if val := rs.ChartTemplate().CodeDigits(); val != 0 {
				data.SetCodeDigits(val)
			}
			if val := rs.ChartTemplate().BankAccountCodePrefix(); val != "" {
				data.SetBankAccountCodePrefix(val)
			}
			if val := rs.ChartTemplate().CashAccountCodePrefix(); val != "" {
				data.SetCashAccountCodePrefix(val)
			}

			return data
		})

	h.WizardMultiChartsAccounts().Methods().GetDefaultBankAccountIds().DeclareMethod(
		`GetDefaultBankAccountIds`,
		func(rs m.WizardMultiChartsAccountsSet) m.AccountBankAccountsWizardSet {
			//@api.model
			/*def _get_default_bank_account_ids(self):
			  return [{'acc_name': _('Cash'), 'account_type': 'cash'}, {'acc_name': _('Bank'), 'account_type': 'bank'}]
				tovalid shall we return a set (hence adding data to database) or slice of data?
			*/
			return h.AccountBankAccountsWizard().NewSet(rs.Env())
		})

	h.WizardMultiChartsAccounts().Methods().DefaultGet().Extend("",
		func(rs m.WizardMultiChartsAccountsSet) m.WizardMultiChartsAccountsData {
			var chartTemplates m.AccountChartTemplateSet
			var chartID int64
			var chart m.AccountChartTemplateSet
			var chartHierarchies m.AccountChartTemplateSet
			var baseTaxCondition q.AccountTaxTemplateCondition
			var saleTax m.AccountTaxTemplateSet
			var purchaseTax m.AccountTaxTemplateSet

			res := rs.Super().DefaultGet()
			if res.HasBankAccounts() {
				res.SetBankAccounts(rs.GetDefaultBankAccountIds())
			}
			if res.HasCompany() {
				res.SetCompany(h.User().NewSet(rs.Env()).CurrentUser().Company())
			}
			if res.HasCurrency() {
				if res.Company().IsNotEmpty() {
					currency := res.Company().OnChangeCountry().Currency()
					res.SetCurrency(currency)
				}
			}

			chartTemplates = h.AccountChartTemplate().Search(rs.Env(), q.AccountChartTemplate().Visible().Equals(true))
			if chartTemplates.IsNotEmpty() {
				// in order to set default chart which was last created set max of ids.
				for _, id := range chartTemplates.Ids() {
					if id > chartID {
						chartID = id
					}
				}
				/*
					if context.get("default_charts"):
					   model_data = self.env['ir.model.data'].search_read([('model', '=', 'account.chart.template'), ('module', '=', context.get("default_charts"))], ['res_id'])
					   if model_data:    //tovalid  ^^^ ir.model.data hexya?
					      chart_id = model_data[0]['res_id']
				*/
				chart = h.AccountChartTemplate().BrowseOne(rs.Env(), chartID)
				chartHierarchies = rs.GetChartParents(chart)
				res.SetOnlyOneChartTemplate(chartTemplates.Len() == 1)
				res.SetChartTemplate(chart)
				baseTaxCondition = q.AccountTaxTemplate().ChartTemplate().In(chartHierarchies)
				saleTax = h.AccountTaxTemplate().Search(rs.Env(), baseTaxCondition.And().TypeTaxUse().Equals("sale")).
					Limit(1).OrderBy("sequence")
				if saleTax.IsNotEmpty() {
					res.SetSaleTax(saleTax)
				}
				purchaseTax = h.AccountTaxTemplate().Search(rs.Env(), baseTaxCondition.And().TypeTaxUse().Equals("purchase")).
					Limit(1).OrderBy("sequence")
				if purchaseTax.IsNotEmpty() {
					res.SetPurchaseTax(purchaseTax)
				}
			}
			res.SetPurchaseTaxRate(15.0)
			res.SetSaleTaxRate(15.0)
			return res
		})

	h.WizardMultiChartsAccounts().Methods().FieldsViewGet().Extend("",
		func(rs m.WizardMultiChartsAccountsSet, args webdata.FieldsViewGetParams) *webdata.FieldsViewData {
			res := rs.Super().FieldsViewGet(args)
			companies := h.Company().Search(rs.Env(), q.CompanyCondition{})
			condition := q.AccountAccount().Deprecated().Equals(false).
				And().Name().NotEquals("Chart For Automated Tests").
				AndNotCond(q.AccountAccount().Name().Like("%(test)"))
			var configuredCmp m.CompanySet
			for _, cmp := range h.AccountAccount().Search(rs.Env(), condition).Records() {
				configuredCmp = configuredCmp.Union(cmp.Company())
			}
			unconfiguredCmp := companies.Subtract(configuredCmp)
			if _, ok := res.Fields["Company"]; ok {
				res.Fields["Company"].Domain = q.WizardMultiChartsAccounts().ID().In(unconfiguredCmp.Ids())
				res.Fields["Company"].Selection = types.Selection{}
				if unconfiguredCmp.Len() > 0 {
					selectionMap := make(types.Selection)
					for _, line := range h.Company().Browse(rs.Env(), unconfiguredCmp.Ids()).Records() {
						selectionMap[strconv.Itoa(int(line.ID()))] = line.Name()
					}
					res.Fields["Company"].Selection = selectionMap
				}
			}
			return res
		})

	h.WizardMultiChartsAccounts().Methods().CreateTaxTemplatesFromRates().DeclareMethod(
		`CreateTaxTemplatesFromRates
			  This function checks if the chosen chart template is configured as containing a full set of taxes, and if
			  it's not the case, it creates the templates for account.tax object accordingly to the provided sale/purchase rates.
			  Then it saves the new tax templates as default taxes to use for this chart template.

			  :param company_id: id of the company for which the wizard is running
			  :return: True`,
		func(rs m.WizardMultiChartsAccountsSet, company m.CompanySet) bool {
			var allParents m.AccountChartTemplateSet
			var value float64
			var cond q.AccountTaxTemplateCondition
			var refTaxs m.AccountTaxTemplateSet

			allParents = rs.GetChartParents(rs.ChartTemplate())
			// create tax templates from purchase_tax_rate and sale_tax_rate fields
			if rs.ChartTemplate().CompleteTaxSet() {
				return true
			}
			value = rs.SaleTaxRate()

			cond = q.AccountTaxTemplate().TypeTaxUse().Equals("sale").
				And().ChartTemplate().In(allParents)
			refTaxs = h.AccountTaxTemplate().Search(rs.Env(), cond).OrderBy("sequence", "id desc").Limit(1)
			refTaxs.Write(h.AccountTaxTemplate().NewData().
				SetAmount(value).
				SetName(rs.T(`Tax %.2f%%`, value)).
				SetDescription(fmt.Sprintf(`%.2f%%`, value)))

			cond = q.AccountTaxTemplate().TypeTaxUse().Equals("purchase").
				And().ChartTemplate().In(allParents)
			refTaxs = h.AccountTaxTemplate().Search(rs.Env(), cond).OrderBy("sequence", "id desc").Limit(1)
			refTaxs.Write(h.AccountTaxTemplate().NewData().
				SetAmount(value).
				SetName(rs.T(`Tax %.2f%%`, value)).
				SetDescription(fmt.Sprintf(`%.2f%%`, value)))
			return true
		})

	h.WizardMultiChartsAccounts().Methods().Execute().DeclareMethod(
		`Execute This function is called at the confirmation of the wizard to generate the COA from the templates. It will read
			  all the provided information to create the accounts, the banks, the journals, the taxes, the
			  accounting properties... accordingly for the chosen company.`,
		func(rs m.WizardMultiChartsAccountsSet) bool {
			if !h.User().NewSet(rs.Env()).CurrentUser().IsAdmin() {
				panic(rs.T(`Only administrators can change the settings.`))
			}

			if h.AccountAccount().Search(rs.Env(), q.AccountAccount().Company().Equals(rs.Company())).Len() > 0 {
				// We are in a case where we already have some accounts existing, meaning that user has probably
				// created its own accounts and does not need a coa, so skip installation of coa.
				log.Info("Could not install chart of account since some accounts already exists for the company", "Company", rs.Company().Name())
				return true
			}

			company := rs.Company()
			rs.Company().Write(h.Company().NewData().
				SetCurrency(rs.Currency()).
				SetAccountsCodeDigits(rs.CodeDigits()).
				SetAngloSaxonAccounting(rs.UseAngloSaxon()).
				SetBankAccountCodePrefix(rs.BankAccountCodePrefix()).
				SetCashAccountCodePrefix(rs.CashAccountCodePrefix()).
				SetChartTemplate(rs.ChartTemplate()))

			// set the coa currency to active
			rs.Currency().Write(h.Currency().NewData().
				SetActive(true))

			// When we install the CoA of first company, set the currency to price types and pricelists
			if company.ID() == 1 {
				for _, reference := range []string{"product_list_price", "product_standard_price", "product_list0"} {
					set := h.ProductProduct().Search(rs.Env(), q.ProductProduct().HexyaExternalID().Equals(reference))
					if set.IsNotEmpty() {
						set.SetCurrency(rs.Currency())
					}
				}
			}

			// If the floats for sale/purchase rates have been filled, create templates from them
			rs.CreateTaxTemplatesFromRates(company)

			// Install all the templates objects and generate the real objects
			accTemplateRef, _ := rs.ChartTemplate().InstallTemplate(company, rs.CodeDigits(), rs.TransferAccount(), h.WizardMultiChartsAccounts().NewSet(rs.Env()), nil, nil)

			// write values of default taxes for product as super user
			/*
			  ir_values_obj = self.env['ir.values'] tovalid missing ir.values in hexya
			  if self.sale_tax_id and taxes_ref:
			      ir_values_obj.sudo().set_default('product.template', "taxes_id", [taxes_ref[self.sale_tax_id.id]], for_all_users=True, company_id=company.id)
			  if self.purchase_tax_id and taxes_ref:
			      ir_values_obj.sudo().set_default('product.template', "supplier_taxes_id", [taxes_ref[self.purchase_tax_id.id]], for_all_users=True, company_id=company.id)

			*/

			// Create Bank journals
			rs.CreateBankJournalsFromO2m(company, accTemplateRef)

			// Create the current year earning account if it wasn't present in the CoA
			unaffectedEarningsXml := h.AccountAccountType().NewSet(rs.Env()).GetRecord("account_data_unaffected_earnings")
			if unaffectedEarningsXml.IsNotEmpty() && h.AccountAccount().Search(rs.Env(),
				q.AccountAccount().Company().Equals(company).And().UserType().Equals(unaffectedEarningsXml)).IsEmpty() {

				h.AccountAccount().Create(rs.Env(), h.AccountAccount().NewData().
					SetCode("999999").
					SetName(rs.T(`Undistributed Profits/Losses`)).
					SetUserType(unaffectedEarningsXml).
					SetCompany(company))
			}
			return true
		})

	h.WizardMultiChartsAccounts().Methods().CreateBankJournalsFromO2m().DeclareMethod(
		`CreateBankJournalsFromO2m
			  This function creates bank journals and its accounts for each line encoded in the field bank_account_ids of the
			  wizard (which is currently only used to create a default bank and cash journal when the CoA is installed).

			  :param company: the company for which the wizard is running.
			  :param acc_template_ref: the dictionary containing the mapping between the ids of account templates and the ids
			      of the accounts that have been generated from them.`,
		func(rs m.WizardMultiChartsAccountsSet, company m.CompanySet, accTemplateRef map[int64]int64) {
			rs.EnsureOne()
			// Create the journals that will trigger the account.account creation
			for _, acc := range rs.BankAccounts().Records() {
				h.AccountJournal().Create(rs.Env(), h.AccountJournal().NewData().
					SetName(acc.AccName()).
					SetType(acc.AccountType()).
					SetCompany(company).
					SetCurrency(acc.Currency()).
					SetSequence(10))
			}
		})

	h.AccountBankAccountsWizard().DeclareTransientModel()

	h.AccountBankAccountsWizard().AddFields(map[string]models.FieldDefinition{
		"AccName": models.CharField{
			String:   "Account Name",
			Required: true},
		"BankAccount": models.Many2OneField{
			RelationModel: h.WizardMultiChartsAccounts(),
			Required:      true,
			OnDelete:      models.Cascade},
		"Currency": models.Many2OneField{
			String:        "Account Currency",
			RelationModel: h.Currency(),
			Help:          "Forces all moves for this account to have this secondary currency."},
		"AccountType": models.SelectionField{
			Selection: types.Selection{
				"cash": "Cash",
				"bank": "Bank"}},
	})

	h.AccountReconcileModelTemplate().DeclareModel()

	h.AccountReconcileModelTemplate().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:   "Button Label",
			Required: true},
		"Sequence": models.IntegerField{
			String:   "Sequence",
			Required: true,
			Default:  models.DefaultValue(10)},
		"HasSecondLine": models.BooleanField{
			String:  "Add a second line",
			Default: models.DefaultValue(false)},
		"Account": models.Many2OneField{
			String:        "Account",
			RelationModel: h.AccountAccountTemplate(),
			OnDelete:      models.Cascade},
		"Label": models.CharField{
			String: "Journal Item Label"},
		"AmountType": models.SelectionField{
			Selection: types.Selection{
				"fixed":      "Fixed",
				"percentage": "Percentage of balance",
			}},
		"Amount": models.FloatField{
			Required: true,
			Default:  models.DefaultValue(100.0),
			Help:     "Fixed amount will count as a debit if it is negative, as a credit if it is positive."},
		"Tax": models.Many2OneField{
			RelationModel: h.AccountTaxTemplate(),
			OnDelete:      models.Restrict,
			Filter:        q.AccountTaxTemplate().TypeTaxUse().Equals("purchase")},
		"SecondAccount": models.Many2OneField{
			RelationModel: h.AccountAccountTemplate(),
			OnDelete:      models.Cascade},
		"SecondLabel": models.CharField{
			String: "Second Journal Item Label"},
		"SecondAmountType": models.SelectionField{
			Selection: types.Selection{
				"fixed":      "Fixed",
				"percentage": "Percentage of amount"}},
		"SecondAmount": models.FloatField{
			Required: true,
			Default:  models.DefaultValue(100.0),
			Help:     "Fixed amount will count as a debit if it is negative, as a credit if it is positive."},
		"SecondTax": models.Many2OneField{
			String:        "Second Tax",
			RelationModel: h.AccountTaxTemplate(),
			OnDelete:      models.Restrict,
			Filter:        q.AccountTaxTemplate().TypeTaxUse().Equals("purchase")},
	})

}
