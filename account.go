// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/hexya-addons/account/accounttypes"
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/i18n"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func CoalesceStr(lst ...string) string {
	for _, str := range lst {
		if str != "" {
			return str
		}
	}
	return ""
}

func CoalesceInt(lst ...int) int {
	for _, nb := range lst {
		if nb != 0 {
			return nb
		}
	}
	return 0
}

func FormatLang(env models.Environment, value float64, currency models.RecordSet) string {
	if currency.IsEmpty() || currency.ModelName() != "Currency" {
		panic("Error while formatting float. the model given is not a Currency model")
	}
	ctx := env.Context()
	locale := i18n.GetLocale(ctx.GetString("lang"))
	curColl := currency.Collection()
	digits := CoalesceInt(curColl.Get("DecimalPlaces").(int), 2)
	if ctx.Get("digits") != nil {
		digits = int(ctx.GetInteger("digits"))
	}
	grouping := CoalesceStr(ctx.GetString("grouping"), locale.Grouping, "[3,0]")
	groupingSpl := strings.Split(strings.TrimSuffix(strings.TrimPrefix(grouping, "["), "]"), ",")
	groupingLeft, err := strconv.Atoi(groupingSpl[0])
	if err != nil {
		groupingLeft = 3
	}
	groupingRight, err := strconv.Atoi(groupingSpl[1])
	if err != nil {
		groupingRight = 0
	}
	separator := CoalesceStr(ctx.GetString("separator"), locale.DecimalPoint, ".")
	thSeparator := CoalesceStr(ctx.GetString("th_separator"), locale.ThousandsSep, ",")
	symbol := CoalesceStr(ctx.GetString("symbol"), curColl.Get("Symbol").(string), "$")
	symPos := CoalesceStr(ctx.GetString("sym_pos"), curColl.Get("Position").(string), "before")
	symToLeft := false
	if symPos == "before" {
		symToLeft = true
	}

	//return strutils.FormatMonetary(value, digits, groupingLeft, groupingRight, separator, thSeparator, symbol, symToLeft)
	// FIXME
	fmt.Println(value, digits, groupingLeft, groupingRight, separator, thSeparator, symbol, symToLeft)
	return ""
}

func init() {

	h.AccountAccountType().DeclareModel()
	h.AccountAccountType().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:    "Account Type",
			Required:  true,
			Translate: true},
		"IncludeInitialBalance": models.BooleanField{
			String: "Bring Accounts Balance Forward",
			Help: `Used in reports to know if we should consider journal items from the beginning of time instead of
from the fiscal year only. Account types that should be reset to zero at each new fiscal year
(like expenses, revenue..) should not have this option set.`},
		"Type": models.SelectionField{
			String: "Type",
			Selection: types.Selection{
				"other":      "Regular",
				"receivable": "Receivable",
				"payable":    "Payable",
				"liquidity":  "Liquidity"},
			Required: true,
			Default:  models.DefaultValue("other"),
			Help: `The 'Internal Type' is used for features available on different types of accounts:
- liquidity type is for cash or bank accounts
- payable/receivable is for vendor/customer accounts.`},
		"Note": models.TextField{
			String: "Description"},
	})

	h.AccountAccountTag().DeclareModel()
	h.AccountAccountTag().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:   "Name",
			Required: true},
		"Applicability": models.SelectionField{
			String: "Applicability",
			Selection: types.Selection{
				"accounts": "Accounts",
				"taxes":    "Taxes"},
			Required: true,
			Default:  models.DefaultValue("accounts")},
		"Color": models.IntegerField{
			String: "Color Index"},
	})

	h.AccountAccount().DeclareModel()
	h.AccountAccount().SetDefaultOrder("Code")

	h.AccountAccount().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			Required: true,
			Index:    true},
		"Currency": models.Many2OneField{
			String:        "Account Currency",
			RelationModel: h.Currency(),
			Help:          "Forces all moves for this account to have this account currency."},
		"Code": models.CharField{
			Size:     64,
			Required: true,
			Index:    true},
		"Deprecated": models.BooleanField{
			Index:   true,
			Default: models.DefaultValue(false)},
		"UserType": models.Many2OneField{
			String:        "Type",
			RelationModel: h.AccountAccountType(),
			Required:      true,
			Help: `Account Type is used for information purpose, to generate country-specific
legal reports, and set the rules to close a fiscal year and generate opening entries.`},
		"InternalType": models.SelectionField{
			Related:    "UserType.Type",
			ReadOnly:   true,
			Constraint: h.AccountAccount().Methods().CheckReconcile(),
			OnChange:   h.AccountAccount().Methods().OnchangeInternalType()},
		"LastTimeEntriesChecked": models.DateTimeField{
			String:   "Latest Invoices & Payments Matching Date",
			ReadOnly: true,
			NoCopy:   true,
			Help: `Last time the invoices & payments matching was performed on this account.
It is set either if there's not at least an unreconciled debit and an unreconciled credit
or if you click the "Done" button.`},
		"Reconcile": models.BooleanField{
			String:     "Allow Reconciliation",
			Default:    models.DefaultValue(false),
			Constraint: h.AccountAccount().Methods().CheckReconcile(),
			Help:       "Check this box if this account allows invoices & payments matching of journal items."},
		"Taxes": models.Many2ManyField{
			String:        "Default Taxes",
			RelationModel: h.AccountTax(),
			JSON:          "tax_ids"},
		"Note": models.TextField{
			String: "Internal Notes"},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Required:      true,
			Default: func(env models.Environment) interface{} {
				return h.Company().NewSet(env).CompanyDefaultGet()
			}},
		"Tags": models.Many2ManyField{
			RelationModel: h.AccountAccountTag(),
			JSON:          "tag_ids",
			Help:          "Optional tags you may want to assign for custom reporting"},
	})

	h.AccountAccount().AddSQLConstraint(
		"code_company_uniq",
		"unique (code,company_id)",
		"The code of the account must be unique per company !")

	h.AccountAccount().Methods().GetDefaultAccountFromChart().DeclareMethod(
		`GetDefaultAccountFromChart returns the default account with the given name from the installed chart of account. 
		If there is no chart of account installed, then a dummy account is returned (and created if necessary)`,
		func(rs m.AccountAccountSet, name string) m.AccountAccountSet {
			company := h.Company().BrowseOne(rs.Env(), rs.Env().Context().GetInteger("force_company"))
			if company.IsEmpty() {
				company = h.User().NewSet(rs.Env()).CurrentUser().Company()
				if company.IsEmpty() {
					return nil
				}
			}
			tmplExternalID := company.ChartTemplate().Get(name).(models.RecordSet).Collection().Wrap().(m.AccountAccountTemplateSet).HexyaExternalID()
			if tmplExternalID == "" {
				return nil
			}
			return h.AccountAccount().NewSet(rs.Env()).GetRecord(fmt.Sprintf("%d_%s", company.ID(), tmplExternalID))

		})

	h.AccountAccount().Methods().CheckReconcile().DeclareMethod(
		`CheckReconcile`,
		func(rs m.AccountAccountSet) {
			for _, r := range rs.Records() {
				if (r.InternalType() == "receivable" || r.InternalType() == "payable") && r.Reconcile() == false {
					panic(rs.T(`You cannot have a recievable/payable account that is not reconciliable. (account code: %s)`, r.Code()))
				}
			}
		})

	h.AccountAccount().Methods().DefaultGet().Extend(
		`If we're creating a new account through a many2one, there are chances that we typed the account code
	instead of its name. In that case, switch both fields values.`,
		func(rs m.AccountAccountSet) m.AccountAccountData {
			defaultName := rs.Env().Context().GetString("default_name")
			defaultCode := rs.Env().Context().GetInteger("default_code") //int??

			if defaultName != "" && defaultCode == 0 {
				i := -789098765
				i, err := strconv.Atoi(defaultName)
				if err == nil && i != -789098765 {
					defaultName = ""
					defaultCode = int64(i)
				}
			}
			return rs.WithContext(defaultName, defaultCode).Super().DefaultGet()
		})

	h.AccountAccount().Methods().SearchByName().Extend("",
		func(rs m.AccountAccountSet, name string, op operator.Operator, additionalCond q.AccountAccountCondition, limit int) m.AccountAccountSet {
			//Tovalid
			//@api.model
			/*def name_search(self, name, args=None, operator='ilike', limit=100):
			  args = args or []
			  domain = []
			  if name:
			      domain = ['|', ('code', '=ilike', name + '%'), ('name', operator, name)]
			      if operator in expression.NEGATIVE_TERM_OPERATORS:
			          domain = ['&', '!'] + domain[1:]
			  accounts = self.search(domain + args, limit=limit)
			  return accounts.name_get()

			*/
			return rs.Super().SearchByName(name, op, additionalCond, limit)
		})

	h.AccountAccount().Methods().OnchangeInternalType().DeclareMethod(
		`OnchangeInternalType`,
		func(rs m.AccountAccountSet) m.AccountAccountData {
			res := h.AccountAccount().NewData()
			if rs.InternalType() == "receivable" || rs.InternalType() == "payable" {
				res.SetReconcile(true)
			} else if rs.InternalType() == "liquidity" {
				res.SetReconcile(false)
			}
			return res
		})

	h.AccountAccount().Methods().NameGet().Extend("",
		func(rs m.AccountAccountSet) string {
			var names string
			for _, r := range rs.Records() {
				name := r.Code() + " " + r.Name()
				names = strings.Join([]string{names, name}, " / ")
			}
			return names
		})

	h.AccountAccount().Methods().Copy().Extend("",
		func(rs m.AccountAccountSet, overrides m.AccountAccountData) m.AccountAccountSet {
			overrides.SetCode(rs.T(`%s (copy)`, rs.Code()))
			return rs.Super().Copy(overrides)
		})

	h.AccountAccount().Methods().Write().Extend("",
		func(rs m.AccountAccountSet, vals m.AccountAccountData) bool {
			// Dont allow changing the company_id when account_move_line already exist
			if vals.HasCompany() {
				query := q.AccountMoveLine().Account().In(rs)
				moveLines := h.AccountMoveLine().Search(rs.Env(), query).Limit(1)
				for _, acc := range rs.Records() {
					if acc.Company() != vals.Company() && !moveLines.IsEmpty() {
						panic(rs.T(`You cannot change the owner company of an account that already contains journal items.`))
					}
				}
			}
			// If user change the reconcile flag, all aml should be recomputed for that account and this is very costly.
			// So to prevent some bugs we add a constraint saying that you cannot change the reconcile field if there is any aml existing
			// for that account.
			if vals.Reconcile() {
				query := q.AccountMoveLine().Account().In(rs)
				moveLines := h.AccountMoveLine().Search(rs.Env(), query).Limit(1)
				if moveLines.Len() > 0 {
					panic(rs.T(`You cannot change the owner company of an account that already contains journal items.`))
				}
			}
			if !vals.Currency().IsEmpty() {

				for _, acc := range rs.Records() {
					query := q.AccountMoveLine().Account().Equals(acc).And().Currency().NotIn(vals.Currency()).And().Currency().IsNotNull()
					if h.AccountMoveLine().Search(rs.Env(), query).Len() > 0 {
						panic(rs.T(`You cannot set a currency on this account as it already has some journal entries having a different foreign currency.`))
					}
				}
			}
			return rs.Super().Write(vals)
		})

	h.AccountAccount().Methods().Unlink().Extend("",
		func(rs m.AccountAccountSet) int64 {
			query := q.AccountMoveLine().Account().In(rs)
			if !h.AccountMoveLine().Search(rs.Env(), query).IsEmpty() {
				panic(rs.T(`You cannot do that on an account that contains journal items.`))
			}
			//Checking whether the account is set as a property to any Partner or not
			var values []string
			for _, id := range rs.Ids() {
				values = append(values, fmt.Sprintf("account.account,%d", id))
			}
			//query = q.Property().ValueReference().In(values)
			//partnerPropAcc := rs.Env().Pool("Property").Search(&query)
			//if partnerPropAcc.Len() > 0 {                                                                    tovalid
			//	panic(rs.T(`You cannot remove/deactivate an account which is set on a customer or vendor.`))
			//}
			return rs.Super().Unlink()
		})

	h.AccountAccount().Methods().MarkAsReconciled().DeclareMethod(
		`MarkAsReconciled`,
		func(rs m.AccountAccountSet) bool {
			return rs.Write(rs.First().SetLastTimeEntriesChecked(dates.Now()))
		})

	h.AccountAccount().Methods().ActionOpenReconcile().DeclareMethod(
		`ActionOpenReconcile`,
		func(rs m.AccountAccountSet) *actions.Action {
			rs.EnsureOne()
			var ctx *types.Context
			ctx = ctx.WithKey("show_mode_selector", false)
			// Open reconciliation view for this account
			if rs.InternalType() == `payable` {
				ctx = ctx.WithKey("mode", "suppliers")
			} else if rs.InternalType() == `receivable` {
				ctx = ctx.WithKey("mode", "customers")
			} else {
				ctx = ctx.WithKey("account_ids", []int64{rs.ID()})
			}
			return &actions.Action{
				Type:    actions.ActionCloseWindow,
				Tag:     "manual_reconciliation_view",
				Context: ctx,
			}
		})

	h.AccountJournal().DeclareModel()
	h.AccountJournal().SetDefaultOrder("Sequence", "Type", "Code")

	h.AccountJournal().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:   "Journal Name",
			Required: true},
		"Code": models.CharField{
			String:   "Short Code",
			Size:     5,
			Required: true,
			Help:     "The journal entries of this journal will be named using this prefix."},
		"Type": models.SelectionField{
			Selection: types.Selection{
				"sale":     "Sale",
				"purchase": "Purchase",
				"cash":     "Cash",
				"bank":     "Bank",
				"general":  "Miscellaneous"},
			Required:   true,
			Constraint: h.AccountJournal().Methods().CheckBankAccount(),
			Help: `Select 'Sale' for customer invoices journals.
Select 'Purchase' for vendor bills journals.
Select 'Cash' or 'Bank' for journals that are used in customer or vendor payments.
Select 'General' for miscellaneous operations journals.`},
		"TypeControls": models.Many2ManyField{
			String:        "Account Types Allowed",
			RelationModel: h.AccountAccountType(),
			JSON:          "type_control_ids"},
		"AccountControls": models.Many2ManyField{
			String:        "Accounts Allowed",
			RelationModel: h.AccountAccount(),
			JSON:          "account_control_ids",
			Filter:        q.AccountAccount().Deprecated().Equals(false)},
		"DefaultCreditAccount": models.Many2OneField{
			RelationModel: h.AccountAccount(),
			Constraint:    h.AccountJournal().Methods().CheckCurrency(),
			OnChange:      h.AccountJournal().Methods().OnchangeCreditAccountId(),
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Help:          "It acts as a default account for credit amount"},
		"DefaultDebitAccount": models.Many2OneField{
			RelationModel: h.AccountAccount(),
			Constraint:    h.AccountJournal().Methods().CheckCurrency(),
			OnChange:      h.AccountJournal().Methods().OnchangeDebitAccountId(),
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Help:          "It acts as a default account for debit amount"},
		"UpdatePosted": models.BooleanField{
			String: "Allow Cancelling Entries",
			Help: `Check this box if you want to allow the cancellation the entries related to this journal or
of the invoice related to this journal`},
		"GroupInvoiceLines": models.BooleanField{
			Help: `If this box is checked, the system will try to group the accounting lines when generating
them from invoices.`},
		"EntrySequence": models.Many2OneField{
			RelationModel: h.Sequence(),
			JSON:          "sequence_id",
			Help:          "This field contains the information related to the numbering of the journal entries of this journal.",
			Required:      true,
			NoCopy:        true},
		"RefundEntrySequence": models.Many2OneField{
			RelationModel: h.Sequence(),
			JSON:          "refund_sequence_id",
			Help:          "This field contains the information related to the numbering of the refund entries of this journal.",
			NoCopy:        true},
		"Sequence": models.IntegerField{
			Help:    "Used to order Journals in the dashboard view', default=10",
			Default: models.DefaultValue(10)},
		"Currency": models.Many2OneField{
			RelationModel: h.Currency(),
			Constraint:    h.AccountJournal().Methods().CheckCurrency(),
			Help:          "The currency used to enter statement"},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Required:      true,
			Index:         true,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser().Company()
			},
			Help: "Company related to this journal"},
		"RefundSequence": models.BooleanField{
			String: "Dedicated Refund Sequence",
			Help: `Check this box if you don't want to share the
same sequence for invoices and refunds made from this journal`,
			Default: models.DefaultValue(false)},
		"InboundPaymentMethods": models.Many2ManyField{
			String:           "Debit Methods",
			RelationModel:    h.AccountPaymentMethod(),
			M2MLinkModelName: "AccountJournalInboundPaymentMethodRel",
			M2MOurField:      "Journal",
			M2MTheirField:    "InboundPaymentMethod",
			JSON:             "inbound_payment_method_ids",
			Filter:           q.AccountPaymentMethod().PaymentType().Equals("inbound"),
			Default: func(env models.Environment) interface{} {
				return h.AccountPaymentMethod().NewSet(env).GetRecord("account_account_payment_method_manual_in")
			},
			Help: `Means of payment for collecting money.
Hexya modules offer various payments handling facilities,
but you can always use the 'Manual' payment method in order
to manage payments outside of the software.`},
		"OutboundPaymentMethods": models.Many2ManyField{
			String:           "Payment Methods",
			M2MLinkModelName: "AccountJournalOutboundPaymentMethodRel",
			M2MOurField:      "Journal",
			M2MTheirField:    "OutboundPaymentMethod",
			RelationModel:    h.AccountPaymentMethod(),
			JSON:             "outbound_payment_method_ids",
			Filter:           q.AccountPaymentMethod().PaymentType().Equals("outbound"),
			Default: func(env models.Environment) interface{} {
				return h.AccountPaymentMethod().NewSet(env).GetRecord("account_account_payment_method_manual_out")
			},
			Help: `Means of payment for sending money.
Hexya modules offer various payments handling facilities
but you can always use the 'Manual' payment method in order
to manage payments outside of the software.`},
		"AtLeastOneInbound": models.BooleanField{
			Compute: h.AccountJournal().Methods().MethodsCompute(),
			Depends: []string{"InboundPaymentMethods", "OutboundPaymentMethods"},
			Stored:  true},
		"AtLeastOneOutbound": models.BooleanField{
			Compute: h.AccountJournal().Methods().MethodsCompute(),
			Depends: []string{"InboundPaymentMethods", "OutboundPaymentMethods"},
			Stored:  true},
		"ProfitAccount": models.Many2OneField{
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Help:          "Used to register a profit when the ending balance of a cash register differs from what the system computes"},
		"LossAccount": models.Many2OneField{
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Help:          "Used to register a loss when the ending balance of a cash register differs from what the system computes"},
		"BelongsToCompany": models.BooleanField{
			String:  "Belong to the user's current company",
			Compute: h.AccountJournal().Methods().BelongToCompany() /*[ search "_search_company_journals"]*/},
		"BankAccount": models.Many2OneField{
			RelationModel: h.BankAccount(),
			OnDelete:      models.Restrict,
			Constraint:    h.AccountJournal().Methods().CheckBankAccount(),
			NoCopy:        true},
		"DisplayOnFooter": models.BooleanField{
			String: "Show in Invoices Footer",
			Help:   "Display this bank account on the footer of printed documents like invoices and sales orders."},
		"BankStatementsSource": models.SelectionField{
			String: "Bank Feeds",
			Selection: types.Selection{
				"manual": "Record Manually"}},
		"BankAccNumber": models.CharField{Related: "BankAccount.Name"},
		"Bank":          models.Many2OneField{RelationModel: h.Bank(), Related: "BankAccount.Bank"},
	})

	h.AccountJournal().AddSQLConstraint(
		"code_company_uniq",
		"unique (code, name, company_id)",
		"The code and name of the journal must be unique per company !'")

	h.AccountJournal().Methods().CheckCurrency().DeclareMethod(
		`CheckCurrency`,
		func(rs m.AccountJournalSet) {
			if !rs.Currency().IsEmpty() {
				if rs.Currency().Equals(rs.Company().Currency()) {
					panic(rs.T(`Currency field should only be set if the journal's currency is different from the company's. Leave the field blank to use company currency.`))
				}
				if !rs.DefaultCreditAccount().IsEmpty() && rs.DefaultCreditAccount().Currency().ID() != rs.Currency().ID() {
					panic(rs.T(`Configuration error!\nThe currency of the journal should be the same than the default credit account.`))
				}
				if !rs.DefaultDebitAccount().IsEmpty() && rs.DefaultDebitAccount().Currency().ID() != rs.Currency().ID() {
					panic(rs.T(`Configuration error!\nThe currency of the journal should be the same than the default debit account.`))
				}
			}
		})

	h.AccountJournal().Methods().CheckBankAccount().DeclareMethod(
		`CheckBankAccount`,
		func(rs m.AccountJournalSet) {
			if rs.Type() == "bank" && !rs.BankAccount().IsEmpty() {
				if !rs.BankAccount().Company().Equals(rs.Company()) {
					panic(rs.T(`
The Company of the bank account associated to this journal (%s) must be the same as this journal's company
journal.Company: '%s' - ID_%d
journal.BankAccount.Company: '%s' - ID_%d).
`, rs.Name(), rs.Company().Name(), rs.Company().ID(), rs.BankAccount().Company().Name(), rs.BankAccount().Company().ID()))
				}
				// A bank account can belong to a customer/supplier, in which case their partner_id is the customer/supplier.
				// Or they are part of a bank journal and their partner_id must be the company's partner_id.
				if !rs.BankAccount().Partner().Equals(rs.Company().Partner()) {
					panic(rs.T(`
The holder of a journal\'s bank account must be this journal's (%s) company holder.
account holder: '%s' - ID_%d
journalCompany holder: '%s' - ID_%d
`, rs.Name(), rs.BankAccount().Partner(), rs.BankAccount().Partner().ID(), rs.Company().Partner(), rs.Company().Partner().ID()))

				}
			}
		})

	h.AccountJournal().Methods().OnchangeDebitAccountId().DeclareMethod(
		`OnchangeDebitAccountId`,
		func(rs m.AccountJournalSet) m.AccountJournalData {
			res := h.AccountJournal().NewData()
			if rs.DefaultCreditAccount().IsEmpty() {
				rs.SetDefaultCreditAccount(rs.DefaultDebitAccount())
			}
			return res
		})

	h.AccountJournal().Methods().OnchangeCreditAccountId().DeclareMethod(
		`OnchangeCreditAccountId`,
		func(rs m.AccountJournalSet) m.AccountJournalData {
			res := h.AccountJournal().NewData()
			if rs.DefaultDebitAccount().IsEmpty() {
				res.SetDefaultDebitAccount(rs.DefaultCreditAccount())
			}
			return res
		})

	h.AccountJournal().Methods().Unlink().Extend("",
		func(rs m.AccountJournalSet) int64 {
			bankAccounts := h.BankAccount().Browse(rs.Env(), []int64{})
			var BAlist []m.BankAccountSet
			for _, r := range rs.Records() {
				BAlist = append(BAlist, r.BankAccount())
			}
			for _, bAcc := range BAlist {
				accounts := rs.Search(q.AccountJournal().BankAccount().Equals(bAcc))
				if accounts.Subtract(rs).IsEmpty() { //if accounts is subset of rs
					bankAccounts = bankAccounts.Union(bAcc)
				}
			}
			ret := rs.Super().Unlink()
			bankAccounts.Unlink()
			return ret
		})

	h.AccountJournal().Methods().Copy().Extend("",
		func(rs m.AccountJournalSet, overrides m.AccountJournalData) m.AccountJournalSet {
			overrides.SetCode(rs.T("%s (copy)", rs.Code()))
			overrides.SetName(rs.T("%s (copy)", rs.Name()))
			return rs.Super().Copy(overrides)
		})

	h.AccountJournal().Methods().Write().Extend("",
		func(rs m.AccountJournalSet, vals m.AccountJournalData) bool {
			for _, journal := range rs.Records() {
				if !vals.Company().IsEmpty() && !journal.Company().Equals(vals.Company()) {
					if !h.AccountMove().Search(rs.Env(), q.AccountMove().Journal().In(rs)).IsEmpty() {
						panic(rs.T(`This journal already contains items, therefore you cannot modify its company.`))
					}
				}
				if vals.Code() != "" && journal.Code() != vals.Code() {
					if !h.AccountMove().Search(rs.Env(), q.AccountMove().Journal().In(rs)).IsEmpty() {
						panic(rs.T(`This journal already contains items, therefore you cannot modify its short name.`))
					}
					newPrefix := rs.GetSequencePrefix(vals.Code(), false)
					journal.EntrySequence().SetPrefix(newPrefix)
					if !journal.RefundEntrySequence().IsEmpty() {
						newPrefix = rs.GetSequencePrefix(vals.Code(), true)
						journal.RefundEntrySequence().SetPrefix(newPrefix)
					}
				}
				if !vals.Currency().IsEmpty() {
					if vals.DefaultDebitAccount().IsEmpty() && !rs.DefaultDebitAccount().IsEmpty() {
						rs.DefaultDebitAccount().SetCurrency(vals.Currency())
					}
					if vals.DefaultCreditAccount().IsEmpty() && !rs.DefaultCreditAccount().IsEmpty() {
						rs.DefaultCreditAccount().SetCurrency(vals.Currency())
					}
					if !rs.BankAccount().IsEmpty() {
						rs.BankAccount().SetCurrency(vals.Currency())
					}
				}
				if vals.BankAccNumber() != "" && !journal.BankAccount().IsEmpty() {
					panic(rs.T(`You cannot empty the account number once set.\nIf you would like to delete the account number, you can do it from the Bank Accounts list.`))
				}
			}
			result := rs.Super().Write(vals)
			// Create the bank_account_id if necessary
			if vals.BankAccNumber() != "" {
				for _, journal := range rs.Filtered(func(r m.AccountJournalSet) bool { return r.Type() == "bank" && !rs.BankAccount().IsEmpty() }).Records() {
					journal.DefineBankAccount(vals.BankAccNumber(), vals.Bank())
				}
			}
			// Create the relevant refund sequence
			if val := vals.Get("RefundSequence"); val != nil {
				for _, journal := range rs.Filtered(func(r m.AccountJournalSet) bool {
					return (r.Type() == "sale" || r.Type() == "purchase") && !r.RefundEntrySequence().IsEmpty()
				}).Records() {
					jVals := h.AccountJournal().NewData()
					jVals.SetName(journal.Name())
					jVals.SetCompany(journal.Company())
					jVals.SetCode(journal.Code())
					journal.SetRefundEntrySequence(rs.Sudo(security.SuperUserID).CreateSequence(jVals, true))
				}
			}
			return result
		})

	h.AccountJournal().Methods().GetSequencePrefix().DeclareMethod(
		`GetSequencePrefix returns the prefix of the sequence for the given code.`,
		func(rs m.AccountJournalSet, code string, refund bool) string {
			prefix := strings.ToUpper(code)
			if refund {
				prefix = "R" + prefix
			}
			return prefix + "/%(range_year)s/"
		})

	h.AccountJournal().Methods().CreateSequence().DeclareMethod(
		`CreateSequence creates new no_gap entry sequence for every new Journal`,
		func(rs m.AccountJournalSet, vals m.AccountJournalData, refund bool) m.SequenceSet {
			prefix := rs.GetSequencePrefix(vals.Code(), refund)
			name := vals.Name()
			if refund {
				name = rs.T("%s: Refund", name)
			}
			seq := h.Sequence().NewData().
				SetName(name).
				SetImplementation("no_gap").
				SetPrefix(prefix).
				SetPadding(4).
				SetNumberIncrement(1).
				SetUseDateRange(true).
				SetCompany(vals.Company())
			return h.Sequence().Create(rs.Env(), seq)
		})

	h.AccountJournal().Methods().PrepareLiquidityAccount().DeclareMethod(
		`PrepareLiquidityAccount prepares the value to use for the creation of the default debit and credit accounts of a
			  liquidity journal (created through the wizard of generating COA from templates for example).`,
		func(rs m.AccountJournalSet, name string, company m.CompanySet, currency m.CurrencySet, accType string) m.AccountAccountData {
			// Seek the next available number for the account code
			codeDigits := company.AccountsCodeDigits()
			accountCodePrefix := company.BankAccountCodePrefix()
			if accType == "cash" {
				if company.CashAccountCodePrefix() != "" {
					accountCodePrefix = company.CashAccountCodePrefix()
				}
			}
			var (
				flag    bool
				newCode string
			)
			for num := 1; num < 100; num++ {
				newCode = strings.Replace(
					fmt.Sprintf("%-[1]*[2]s%d", codeDigits-1, accountCodePrefix, num), " ", "0", -1)
				rec := h.AccountAccount().Search(rs.Env(),
					q.AccountAccount().Code().Equals(newCode).And().Company().Equals(company)).Limit(1)
				if rec.IsEmpty() {
					flag = true
					break
				}
			}
			if !flag {
				panic(rs.T("Cannot generate an unused account code."))
			}
			liquidityType := h.AccountAccountType().Search(rs.Env(),
				q.AccountAccountType().HexyaExternalID().Equals("account_data_account_type_liquidity"))

			return h.AccountAccount().NewData().
				SetName(name).
				SetCurrency(currency).
				SetCode(newCode).
				SetUserType(liquidityType).
				SetCompany(company)
		})

	h.AccountJournal().Methods().Create().Extend("",
		func(rs m.AccountJournalSet, vals m.AccountJournalData) m.AccountJournalSet {
			company := vals.Company()
			if company.IsEmpty() {
				company = h.User().NewSet(rs.Env()).CurrentUser().Company()
			}
			if vals.Type() == "bank" || vals.Type() == "cash" {
				// # For convenience, the name can be inferred from account number
				// if not vals.get('name') and 'bank_acc_number' in vals:
				//    vals['name'] = vals['bank_acc_number']
				if vals.Code() == "" {
					journalCodeBase := "BNK"
					if vals.Type() == "cash" {
						journalCodeBase = "CSH"
					}
					journals := h.AccountJournal().Search(rs.Env(),
						q.AccountJournal().Code().Like(journalCodeBase+"%").And().Company().Equals(company))
					journalCodes := make(map[string]bool)
					for _, j := range journals.Records() {
						journalCodes[j.Code()] = true
					}
					for num := 1; num < 100; num++ {
						// journal_code has a maximal size of 5, hence we can enforce the boundary num < 100
						jCode := journalCodeBase + strconv.Itoa(num)
						if _, exists := journalCodes[jCode]; !exists {
							vals.SetCode(jCode)
							break
						}
					}
					if vals.Code() == "" {
						panic(rs.T("Cannot generate an unused journal code. Please fill the 'Shortcode' field."))
					}
				}
				// Create a default debit/credit account if not given
				defaultAccount := vals.DefaultDebitAccount()
				if defaultAccount.IsEmpty() {
					defaultAccount = vals.DefaultCreditAccount()
				}
				if defaultAccount.IsEmpty() {
					accountVals := rs.PrepareLiquidityAccount(vals.Name(), company, vals.Currency(), vals.Type())
					defaultAccount = h.AccountAccount().Create(rs.Env(), accountVals)
					vals.SetDefaultDebitAccount(defaultAccount)
					vals.SetDefaultCreditAccount(defaultAccount)
				}

			}
			// We just need to create the relevant sequences according to the chosen options
			if vals.EntrySequence().IsEmpty() {
				vals.SetEntrySequence(rs.Sudo().CreateSequence(vals, false))
			}
			if (vals.Type() == "sale" || vals.Type() == "purchase") && vals.RefundSequence() && vals.RefundEntrySequence().IsEmpty() {
				vals.SetRefundEntrySequence(rs.Sudo().CreateSequence(vals, true))
			}
			journal := rs.Super().Create(vals)

			/*

			  # Create the bank_account_id if necessary
			  if journal.type == 'bank' and not journal.bank_account_id and vals.get('bank_acc_number'):
			      journal.set_bank_account(vals.get('bank_acc_number'), vals.get('bank_id'))

			  return journal

			*/
			return journal
		})

	h.AccountJournal().Methods().DefineBankAccount().DeclareMethod(
		`Create a res.partner.bank and set it as value of the  field bank_account_id`,
		func(rs m.AccountJournalSet, accNumber string, bank m.BankSet) {
			rs.EnsureOne()
			data := h.BankAccount().NewData()
			data.SetSanitizedAccountNumber(accNumber)
			data.SetBank(bank)
			data.SetCompany(rs.Company())
			data.SetCurrency(rs.Currency())
			data.SetPartner(rs.Company().Partner())
			rs.SetBankAccount(h.BankAccount().Create(rs.Env(), data))
		})

	h.AccountJournal().Methods().NameGet().Extend("",
		func(rs m.AccountJournalSet) string {
			currency := rs.Company().Currency()
			if !rs.Currency().IsEmpty() {
				currency = rs.Currency()
			}
			name := fmt.Sprintf(`%s (%s)`, rs.Name(), currency.Name())
			return name
		})

	h.AccountJournal().Methods().SearchByName().Extend("",
		func(rs m.AccountJournalSet, name string, op operator.Operator, additionalCond q.AccountJournalCondition, limit int) m.AccountJournalSet {
			//@api.model
			/*def name_search(self, name, args=None, operator='ilike', limit=100): tovalid
			args = args or []
			connector = '|'
			if operator in expression.NEGATIVE_TERM_OPERATORS:
				connector = '&'
			recs = self.search([connector, ('code', operator, name), ('name', operator, name)] + args, limit=limit)
			return recs.name_get()

			*/
			return rs.Super().SearchByName(name, op, additionalCond, limit)
		})

	h.AccountJournal().Methods().BelongToCompany().DeclareMethod(
		`BelongToCompany`,
		func(rs m.AccountJournalSet) m.AccountJournalData {
			r := h.AccountJournal().NewData()
			r.SetBelongsToCompany(false)
			if rs.Company().Equals(h.User().NewSet(rs.Env()).CurrentUser().Company()) {
				r.SetBelongsToCompany(true)
			}
			return r
		})

	h.AccountJournal().Methods().SearchCompanyJournals().DeclareMethod(
		`SearchCompanyJournals`,
		func(rs m.AccountJournalSet, op operator.Operator, value string) {
			var recs m.AccountJournalSet
			if op == "=" {
				recs = rs.Search(q.AccountJournal().Company().NotEquals(h.User().NewSet(rs.Env()).CurrentUser().Company()))
			} else {
				recs = rs.Search(q.AccountJournal().Company().AddOperator(op, h.User().NewSet(rs.Env()).CurrentUser().Company()))
			}
			_ = recs
			//return [('id', 'in', [x.id for x in recs])] tovalid
		})

	h.AccountJournal().Methods().MethodsCompute().DeclareMethod(
		`MethodsCompute`,
		func(rs m.AccountJournalSet) m.AccountJournalData {
			r := h.AccountJournal().NewData()
			b := len(rs.InboundPaymentMethods().Ids()) > 0
			if b != rs.AtLeastOneInbound() {
				r.SetAtLeastOneInbound(b)
			}
			b = len(rs.OutboundPaymentMethods().Ids()) > 0
			if b != rs.AtLeastOneOutbound() {
				r.SetAtLeastOneOutbound(b)
			}
			return r
		})

	h.BankAccount().AddFields(map[string]models.FieldDefinition{
		"Journal": models.One2ManyField{
			RelationModel: h.AccountJournal(),
			ReverseFK:     "BankAccount",
			JSON:          "journal_id",
			Filter:        q.AccountJournal().Type().Equals("bank"),
			ReadOnly:      true,
			Help:          "The accounting journal corresponding to this bank account.",
			Constraint:    h.BankAccount().Methods().CheckJournal()},
	})

	h.BankAccount().Methods().CheckJournal().DeclareMethod(
		`CheckJournal`,
		func(rs m.BankAccountSet) {
			if rs.Journal().Len() > 1 {
				panic(rs.T(`A bank account can only belong to one journal.`))
			}
		})

	h.AccountTaxGroup().DeclareModel()
	h.AccountTaxGroup().SetDefaultOrder("Sequence ASC")

	h.AccountTaxGroup().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			Required:  true,
			Translate: true},
		"Sequence": models.IntegerField{
			Default: models.DefaultValue(10)},
	})

	h.AccountTax().DeclareModel()
	h.AccountTax().SetDefaultOrder("Sequence")

	h.AccountTax().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:    "Tax Name",
			Required:  true,
			Translate: true},
		"TypeTaxUse": models.SelectionField{
			String: "Tax Scope",
			Selection: types.Selection{
				"sale":     "Sales",
				"purchase": "Purchases",
				"none":     "None"},
			Required:   true,
			Default:    models.DefaultValue("sale"),
			Constraint: h.AccountTax().Methods().CheckChildrenScope(),
			Help: `Determines where the tax is selectable.
Note: 'None' means a tax can't be used by itself however it can still be used in a group.`},
		"TaxAdjustment": models.BooleanField{
			String: "TaxAdjustment",
			Help: `Set this field to true if this tax can be used in the tax adjustment wizard,
used to manually fill some data in the tax declaration`},
		"AmountType": models.SelectionField{
			String: "Tax Computation",
			Selection: types.Selection{
				"group":    "Group of Taxes",
				"fixed":    "Fixed",
				"percent":  "Percentage of Price",
				"division": "Percentage of Price Tax Included"},
			Required: true,
			Default:  models.DefaultValue("percent")},
		"Active": models.BooleanField{
			String:  "Active",
			Default: models.DefaultValue(true),
			Help:    "Set active to false to hide the tax without removing it."},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Required:      true,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser().Company()
			}},
		"ChildrenTaxes": models.Many2ManyField{
			RelationModel: h.AccountTax(),
			JSON:          "children_tax_ids",
			M2MTheirField: "ChildTax",
			M2MOurField:   "ParentTax",
			Constraint:    h.AccountTax().Methods().CheckChildrenScope()},
		"Sequence": models.IntegerField{
			Required: true,
			GoType:   new(int),
			Default:  models.DefaultValue(1),
			Help:     "The sequence field is used to define order in which the tax lines are applied."},
		"Amount": models.FloatField{
			Required: true,
			Digits: nbutils.Digits{
				Precision: 16,
				Scale:     4},
			OnChange: h.AccountTax().Methods().OnchangeAmount()},
		"Account": models.Many2OneField{
			String:        "Tax Account",
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			OnDelete:      models.Restrict,
			OnChange:      h.AccountTax().Methods().OnchangeAccount(),
			Help:          "Account that will be set on invoice tax lines for invoices. Leave empty to use the expense account."},
		"RefundAccount": models.Many2OneField{
			String:        "Tax Account on Refunds",
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			OnDelete:      models.Restrict,
			Help:          "Account that will be set on invoice tax lines for refunds. Leave empty to use the expense account."},
		"Description": models.CharField{
			String:    "Label on Invoices",
			Translate: true},
		"PriceInclude": models.BooleanField{
			String:   "Included in Price",
			Default:  models.DefaultValue(false),
			OnChange: h.AccountTax().Methods().OnchangePriceInclude(),
			Help:     "Check this if the price you use on the product and invoices includes this tax."},
		"IncludeBaseAmount": models.BooleanField{
			String:  "Affect Base of Subsequent Taxes",
			Default: models.DefaultValue(false),
			Help:    "If set, taxes which are computed after this one will be computed based on the price tax included."},
		"Analytic": models.BooleanField{
			String: "Include in Analytic Cost",
			Help: `If set, the amount computed by this tax will be assigned
to the same analytic account as the invoice line (if any)`},
		"Tags": models.Many2ManyField{
			String:        "Tags",
			RelationModel: h.AccountAccountTag(),
			JSON:          "tag_ids",
			Help:          "Optional tags you may want to assign for custom reporting"},
		"TaxGroup": models.Many2OneField{
			RelationModel: h.AccountTaxGroup(),
			Default: func(env models.Environment) interface{} {
				return h.AccountTaxGroup().NewSet(env).SearchAll().Limit(1)
			},
			Required: true},
	})

	// TODO Convert to constrains method
	//h.AccountTax().AddSQLConstraint("name_company_uniq", "unique(name, company_id, type_tax_use)",
	//	"Tax names must be unique !")

	h.AccountTax().Methods().Unlink().Extend("",
		func(rs m.AccountTaxSet) int64 {
			//@api.multi
			/*def unlink(self):
			  company_id = self.env.user.company_id.id
			  ir_values = self.env['ir.values'] tovalid
			  supplier_taxes_id = set(ir_values.get_default('product.template', 'supplier_taxes_id', company_id=company_id) or [])
			  deleted_sup_tax = self.filtered(lambda tax: tax.id in supplier_taxes_id)
			  if deleted_sup_tax:
			      ir_values.sudo().set_default('product.template', "supplier_taxes_id", list(supplier_taxes_id - set(deleted_sup_tax.ids)), for_all_users=True, company_id=company_id)
			  taxes_id = set(self.env['ir.values'].get_default('product.template', 'taxes_id', company_id=company_id) or [])
			  deleted_tax = self.filtered(lambda tax: tax.id in taxes_id)
			  if deleted_tax:
			      ir_values.sudo().set_default('product.template', "taxes_id", list(taxes_id - set(deleted_tax.ids)), for_all_users=True, company_id=company_id)
			  return super(AccountTax, self).unlink()

			*/
			return rs.Super().Unlink()
		})

	h.AccountTax().Methods().CheckChildrenScope().DeclareMethod(
		`CheckChildrenScope`,
		func(rs m.AccountTaxSet) {
			for _, child := range rs.ChildrenTaxes().Records() {
				if !(child.TypeTaxUse() == "none" || child.TypeTaxUse() == rs.TypeTaxUse()) {
					panic(rs.T(`The application scope of taxes in a group must be either the same as the group or "none".`))
				}
			}
		})

	h.AccountTax().Methods().Copy().Extend("",
		func(rs m.AccountTaxSet, overrides m.AccountTaxData) m.AccountTaxSet {
			overrides.SetName(rs.T("%s (Copy)", rs.Name()))
			return rs.Super().Copy(overrides)
		})

	h.AccountTax().Methods().SearchByName().Extend(`SearchByName`,
		func(rs m.AccountTaxSet, name string, op operator.Operator, additionalCond q.AccountTaxCondition, limit int) m.AccountTaxSet {
			/* tovalid
			args = args or []
			if operator in expression.NEGATIVE_TERM_OPERATORS:
				domain = [('description', operator, name), ('name', operator, name)]
			else:
				domain = ['|', ('description', operator, name), ('name', operator, name)]
			taxes = self.search(expression.AND([domain, args]), limit=limit)
			return taxes.name_get()

			*/
			return rs.Super().SearchByName(name, op, additionalCond, limit)
		})

	h.AccountTax().Methods().Search().Extend("",
		func(rs m.AccountTaxSet, cond q.AccountTaxCondition) m.AccountTaxSet {
			ctx := rs.Env().Context()
			typ := ctx.GetString(`type`)
			switch typ {
			case "out_invoice", "out_refund":
				cond = cond.And().TypeTaxUse().Equals("sale")
			case "in_invoice", "in_refund":
				cond = cond.And().TypeTaxUse().Equals("purchase")
			}
			jId := ctx.GetIntegerSlice(`journal_id`)
			if len(jId) > 0 {
				journal := h.AccountJournal().Browse(rs.Env(), jId)
				if journal.Type() == "sale" || journal.Type() == "purchase" {
					cond = cond.And().TypeTaxUse().Equals(journal.Type())
				}
			}
			return rs.Super().Search(cond)
		})

	h.AccountTax().Methods().OnchangeAmount().DeclareMethod(
		`OnchangeAmount`,
		func(rs m.AccountTaxSet) m.AccountTaxData {
			res := h.AccountTax().NewData()
			if (rs.AmountType() == "percent" || rs.AmountType() == "division") && rs.Amount() != 0.0 && rs.Description() == "" {
				res.SetDescription(fmt.Sprintf("%.4f", rs.Amount()))
			}
			return res
		})

	h.AccountTax().Methods().OnchangeAccount().DeclareMethod(
		`OnchangeAccount`,
		func(rs m.AccountTaxSet) m.AccountTaxData {
			res := h.AccountTax().NewData()
			if !rs.RefundAccount().Equals(rs.Account()) {
				res.SetRefundAccount(rs.Account())
			}
			return res
		})

	h.AccountTax().Methods().OnchangePriceInclude().DeclareMethod(
		`OnchangePriceInclude`,
		func(rs m.AccountTaxSet) m.AccountTaxData {
			res := h.AccountTax().NewData()
			if rs.PriceInclude() && !rs.IncludeBaseAmount() {
				res.SetIncludeBaseAmount(true)
			}
			return res
		})

	h.AccountTax().Methods().GetGroupingKey().DeclareMethod(
		`Returns a string that will be used to group account.invoice.tax sharing the same properties`,
		func(rs m.AccountTaxSet, invoiceTaxVal m.AccountInvoiceTaxData) string {
			rs.EnsureOne()
			str := fmt.Sprintf(`%d-%d-%d`, invoiceTaxVal.Tax().ID(), invoiceTaxVal.Account().ID(), invoiceTaxVal.AccountAnalytic().ID())
			return str
		})

	h.AccountTax().Methods().ComputeAmount().DeclareMethod(
		`ComputeAmount returns the amount of a single tax.

		baseAmount is the actual amount on which the tax is applied, which is priceUnit * quantity eventually
		affected by previous taxes (if tax is include_base_amount XOR price_include)`,
		func(rs m.AccountTaxSet, baseAmount, priceUnit, quantity float64, product m.ProductProductSet, partner m.PartnerSet) float64 {
			rs.EnsureOne()
			if rs.AmountType() == "fixed" {
				// Use Copysign to take into account the sign of the base amount which includes the sign
				// of the quantity and the sign of the priceUnit
				// Amount is the fixed price for the tax, it can be negative
				// Base amount included the sign of the quantity and the sign of the unit price and when
				// a product is returned, it can be done either by changing the sign of quantity or by changing the
				// sign of the price unit.
				// When the price unit is equal to 0, the sign of the quantity is absorbed in base_amount then
				// a "else" case is needed.
				if baseAmount != 0 {
					return math.Copysign(quantity, baseAmount) * rs.Amount()
				}
				return quantity * rs.Amount()
			}
			if (rs.AmountType() == "percent" && !rs.PriceInclude()) || (rs.AmountType() == "division" && rs.PriceInclude()) {
				return baseAmount * rs.Amount() / 100
			}
			if rs.AmountType() == "percent" && rs.PriceInclude() {
				return baseAmount - (baseAmount / (1 + rs.Amount()/100))
			}
			if rs.AmountType() == "division" && !rs.PriceInclude() {
				return baseAmount/(1-rs.Amount()/100) - baseAmount
			}
			log.Panic("Unhandled tax type", "tax", rs.ID(), "type", rs.AmountType(), "priceInclude", rs.PriceInclude())
			panic("Unhandled tax type")
		})

	h.AccountTax().Methods().JSONFriendlyComputeAll().DeclareMethod(
		`Just converts parameters in browse records and calls for compute_all, because js widgets can't serialize browse records`,
		func(rs m.AccountTaxSet, priceUnit float64, currencyID int64, quantity float64, productID int64, partnerID int64) (float64, float64, float64, []accounttypes.AppliedTaxData) {
			currency := h.Currency().NewSet(rs.Env())
			if currencyID > 0 {
				currency = h.Currency().Browse(rs.Env(), []int64{currencyID})
			}
			product := h.ProductProduct().NewSet(rs.Env())
			if productID > 0 {
				product = h.ProductProduct().Browse(rs.Env(), []int64{productID})
			}
			partner := h.Partner().NewSet(rs.Env())
			if partnerID > 0 {
				partner = h.Partner().Browse(rs.Env(), []int64{partnerID})
			}
			return rs.ComputeAll(priceUnit, currency, quantity, product, partner)
		})

	h.AccountTax().Methods().ComputeAll().DeclareMethod(
		`ComputeAll returns all information required to apply taxes (in self + their children in case of a tax goup).
			      We consider the sequence of the parent for group of taxes.
			          Eg. considering letters as taxes and alphabetic order as sequence :
			          [G, B([A, D, F]), E, C] will be computed as [A, D, F, C, E, G]

			  RETURN:

                   0.0,                 # Base

			       0.0,                 # Total without taxes

			       0.0,                 # Total with taxes

                   []AppliedTaxData     # One struct for each tax in rs and their children
			  } `,
		func(rs m.AccountTaxSet, priceUnit float64, currency m.CurrencySet, quantity float64,
			product m.ProductProductSet, partner m.PartnerSet) (float64, float64, float64, []accounttypes.AppliedTaxData) {

			company := rs.Company()
			if rs.IsEmpty() {
				company = h.User().NewSet(rs.Env()).CurrentUser().Company()
			}
			if currency == nil || currency.IsEmpty() {
				currency = company.Currency()
			}
			if partner == nil {
				partner = h.Partner().NewSet(rs.Env())
			}
			var taxes []accounttypes.AppliedTaxData
			// By default, for each tax, tax amount will first be computed
			// and rounded at the 'Account' decimal precision for each
			// PO/SO/invoice line and then these rounded amounts will be
			// summed, leading to the total amount for that tax. But, if the
			// company has tax_calculation_rounding_method = round_globally,
			// we still follow the same method, but we use a much larger
			// precision when we round the tax amount for each line (we use
			// the 'Account' decimal precision + 5), and that way it's like
			// rounding after the sum of the tax amounts of each line

			dp := currency.DecimalPlaces()
			// In some cases, it is necessary to force/prevent the rounding of the tax and the total
			// amounts. For example, in SO/PO line, we don't want to round the price unit at the
			// precision of the currency.
			// The context key 'round' allows to force the standard behavior.
			roundTax := true
			if company.TaxCalculationRoundingMethod() == "round_globally" {
				roundTax = false
			}
			roundTotal := true
			if rs.Env().Context().HasKey("round") {
				roundTax = rs.Env().Context().GetBool("round")
				roundTotal = rs.Env().Context().GetBool("round")
			}
			if !roundTax {
				dp += 5
			}
			prec := math.Pow10(-dp)
			totalExcluded := nbutils.Round(priceUnit*quantity, prec)
			totalIncluded := nbutils.Round(priceUnit*quantity, prec)
			base := nbutils.Round(priceUnit*quantity, prec)
			baseValues := rs.Env().Context().GetFloatSlice("base_values")
			if len(baseValues) != 0 {
				totalExcluded = baseValues[0]
				totalIncluded = baseValues[1]
				base = baseValues[2]
			}
			// Sorting key is mandatory in this case. When no key is provided, sorted() will perform a
			// search. However, the search method is overridden in account.tax in order to add a domain
			// depending on the context. This domain might filter out some taxes from self, e.g. in the
			// case of group taxes.
			taxRecords := rs.Records()
			sort.Slice(taxRecords, func(i, j int) bool {
				return taxRecords[i].Sequence() < taxRecords[j].Sequence()
			})
			for _, tax := range taxRecords {
				if tax.AmountType() == "group" {
					children := tax.ChildrenTaxes().WithContext("base_values", []float64{totalExcluded, totalIncluded, base})
					retBase, retExcl, retIncl, retTaxes := children.ComputeAll(priceUnit, currency, quantity, product, partner)
					totalExcluded = retExcl
					if tax.IncludeBaseAmount() {
						base = retBase
					}
					totalIncluded = retIncl
					taxes = append(taxes, retTaxes...)
					continue
				}

				taxAmount := tax.ComputeAmount(base, priceUnit, quantity, product, partner)
				if roundTax {
					taxAmount = nbutils.Round(taxAmount, prec)
				} else {
					taxAmount = currency.Round(taxAmount)
				}

				if tax.PriceInclude() {
					totalExcluded -= taxAmount
					base -= taxAmount
				} else {
					totalIncluded += taxAmount
				}

				// Keep base amount used for the current tax
				taxBase := base

				if tax.IncludeBaseAmount() {
					base += taxAmount
				}

				taxes = append(taxes, accounttypes.AppliedTaxData{
					ID:              tax.ID(),
					Name:            tax.WithContext("lang", partner.Lang()).Name(),
					Amount:          taxAmount,
					Base:            taxBase,
					Sequence:        tax.Sequence(),
					AccountID:       tax.Account().ID(),
					RefundAccountID: tax.RefundAccount().ID(),
					Analytic:        tax.Analytic(),
				})
			}

			if roundTotal {
				totalIncluded = currency.Round(totalIncluded)
				totalExcluded = currency.Round(totalExcluded)
			}
			sort.Slice(taxes, func(i, j int) bool {
				return taxes[i].Sequence < taxes[j].Sequence
			})
			return base, totalExcluded, totalIncluded, taxes
		})

	h.AccountTax().Methods().FixTaxIncludedPrice().DeclareMethod(
		`Subtract tax amount from price when corresponding "price included" taxes do not apply`,
		func(rs m.AccountTaxSet, price float64, prodTaxes, lineTaxes m.AccountTaxSet) float64 {
			// FIXME get currency in param?
			inclTax := prodTaxes.Filtered(func(r m.AccountTaxSet) bool { return r.Intersect(lineTaxes).IsEmpty() && r.PriceInclude() })
			if inclTax.IsNotEmpty() {
				_, totalExcluded, _, _ := inclTax.ComputeAll(price, nil, 1, nil, nil)
				return totalExcluded
			}
			return price
		})

	h.AccountReconcileModel().DeclareModel()

	h.AccountReconcileModel().AddFields(map[string]models.FieldDefinition{
		"Name": models.CharField{
			String:   "Button Label",
			Required: true,
			OnChange: h.AccountReconcileModel().Methods().OnchangeName()},
		"Sequence": models.IntegerField{
			Required: true,
			Default:  models.DefaultValue(10)},
		"HasSecondLine": models.BooleanField{
			String:  "Add a second line",
			Default: models.DefaultValue(false)},
		"Company": models.Many2OneField{
			RelationModel: h.Company(),
			Required:      true,
			Default: func(env models.Environment) interface{} {
				return h.User().NewSet(env).CurrentUser().Company()
			}},
		"Account": models.Many2OneField{
			RelationModel: h.AccountAccount(),
			OnDelete:      models.Cascade,
			Filter:        q.AccountAccount().Deprecated().Equals(false)},
		"Journal": models.Many2OneField{
			RelationModel: h.AccountJournal(),
			OnDelete:      models.Cascade,
			Help:          "This field is ignored in a bank statement reconciliation."},
		"Label": models.CharField{
			String: "Journal Item Label"},
		"AmountType": models.SelectionField{
			Selection: types.Selection{
				"fixed":      "Fixed",
				"percentage": "Percentage of balance"},
			Required: true,
			Default:  models.DefaultValue("percentage")},
		"Amount": models.FloatField{
			Required: true,
			Default:  models.DefaultValue(100.0),
			Help:     "Fixed amount will count as a debit if it is negative, as a credit if it is positive."},
		"Tax": models.Many2OneField{
			String:        "Tax",
			RelationModel: h.AccountTax(),
			OnDelete:      models.Restrict,
			Filter:        q.AccountTax().TypeTaxUse().Equals("purchase")},
		"AnalyticAccount": models.Many2OneField{
			RelationModel: h.AccountAnalyticAccount(),
			OnDelete:      models.SetNull},
		"SecondAccount": models.Many2OneField{
			RelationModel: h.AccountAccount(),
			OnDelete:      models.Cascade,
			Filter:        q.AccountAccount().Deprecated().Equals(false),
		},
		"SecondJournal": models.Many2OneField{
			RelationModel: h.AccountJournal(),
			OnDelete:      models.Cascade,
			Help:          "This field is ignored in a bank statement reconciliation."},
		"SecondLabel": models.CharField{
			String: "Second Journal Item Label"},
		"SecondAmountType": models.SelectionField{
			Selection: types.Selection{
				"fixed":      "Fixed",
				"percentage": "Percentage of balance"},
			Required: true,
			Default:  models.DefaultValue("percentage")},
		"SecondAmount": models.FloatField{
			Required: true,
			Default:  models.DefaultValue(100.0),
			Help:     "Fixed amount will count as a debit if it is negative, as a credit if it is positive."},
		"SecondTax": models.Many2OneField{
			RelationModel: h.AccountTax(),
			OnDelete:      models.Restrict,
			Filter:        q.AccountTax().TypeTaxUse().Equals("purchase")},
		"SecondAnalyticAccount": models.Many2OneField{
			RelationModel: h.AccountAnalyticAccount(),
			OnDelete:      models.SetNull},
	})

	h.AccountReconcileModel().Methods().OnchangeName().DeclareMethod(
		`OnchangeName`,
		func(rs m.AccountReconcileModelSet) m.AccountReconcileModelData {
			res := h.AccountReconcileModel().NewData()
			if rs.Label() != rs.Name() {
				res.SetLabel(rs.Name())
			}
			return res
		})

}
