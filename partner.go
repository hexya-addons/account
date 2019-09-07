package account

import (
	"github.com/hexya-addons/base"
	"github.com/hexya-addons/web/webdata"
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/operator"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/strutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
	"github.com/hexya-erp/pool/q"
)

func init() {

	h.AccountFiscalPosition().DeclareModel()
	h.AccountFiscalPosition().SetDefaultOrder("Sequence")

	h.AccountFiscalPosition().AddFields(map[string]models.FieldDefinition{
		"Sequence": models.IntegerField{},
		"Name": models.CharField{
			String:   "Fiscal Position",
			Required: true},
		"Active": models.BooleanField{
			Default: models.DefaultValue(true),
			Help:    "By unchecking the active field, you may hide a fiscal position without deleting it."},
		"Company": models.Many2OneField{
			RelationModel: h.Company()},
		"Accounts": models.One2ManyField{
			String:        "Account Mapping",
			RelationModel: h.AccountFiscalPositionAccount(),
			ReverseFK:     "Position",
			JSON:          "account_ids",
			Copy:          true},
		"Taxes": models.One2ManyField{
			String:        "Tax Mapping",
			RelationModel: h.AccountFiscalPositionTax(),
			ReverseFK:     "Position",
			JSON:          "tax_ids",
			Copy:          true},
		"Note": models.TextField{
			String:    "Notes",
			Translate: true,
			Help:      "Legal mentions that have to be printed on the invoices."},
		"AutoApply": models.BooleanField{
			String: "Detect Automatically",
			Help:   "Apply automatically this fiscal position."},
		"VatRequired": models.BooleanField{
			String: "VAT required",
			Help:   "Apply only if partner has a VAT number."},
		"Country": models.Many2OneField{
			String:        "Country",
			RelationModel: h.Country(),
			OnChange:      h.AccountFiscalPosition().Methods().OnchangeCountry(),
			Help:          "Apply only if delivery or invoicing country match."},
		"CountryGroup": models.Many2OneField{
			String:        "Country Group",
			RelationModel: h.CountryGroup(),
			OnChange:      h.AccountFiscalPosition().Methods().OnchangeCountryGroup(),
			Help:          "Apply only if delivery or invocing country match the group."},
		"States": models.Many2ManyField{
			String:        "Federal States",
			RelationModel: h.CountryState(),
			JSON:          "state_ids"},
		"ZipFrom": models.CharField{
			String:     "Zip Range From",
			Default:    models.DefaultValue("0"),
			Constraint: h.AccountFiscalPosition().Methods().CheckZip()},
		"ZipTo": models.CharField{
			String:     "Zip Range To",
			Default:    models.DefaultValue("0"),
			Constraint: h.AccountFiscalPosition().Methods().CheckZip()},
		"StatesCount": models.IntegerField{
			Compute: h.AccountFiscalPosition().Methods().ComputeStatesCount(),
			GoType:  new(int)},
	})

	h.AccountFiscalPosition().Methods().ComputeStatesCount().DeclareMethod(
		`ComputeStatesCount returns the number of states of the partner's country'`,
		func(rs m.AccountFiscalPositionSet) m.AccountFiscalPositionData {
			return h.AccountFiscalPosition().NewData().SetStatesCount(rs.Country().States().Len())
		})

	h.AccountFiscalPosition().Methods().CheckZip().DeclareMethod(
		`CheckZip fails if the zip range is the wrong way round`,
		func(rs m.AccountFiscalPositionSet) {
			if rs.ZipFrom() > rs.ZipTo() {
				log.Panic("Invalid 'Zip Range', please configure it properly.")
			}
		})

	h.AccountFiscalPosition().Methods().MapTax().DeclareMethod(
		`MapTax`,
		func(rs m.AccountFiscalPositionSet, taxes m.AccountTaxSet, product m.ProductProductSet,
			partner m.PartnerSet) m.AccountTaxSet {

			result := h.AccountTax().NewSet(rs.Env())
			for _, tax := range taxes.Records() {
				taxCount := 0
				for _, t := range rs.Taxes().Records() {
					if t.TaxSrc().Equals(tax) {
						taxCount++
						if t.TaxDest().IsNotEmpty() {
							result = result.Union(t.TaxDest())
						}
					}
				}
				if taxCount == 0 {
					result = result.Union(tax)
				}
			}
			return result
		})

	h.AccountFiscalPosition().Methods().MapAccount().DeclareMethod(
		`MapAccount`,
		func(rs m.AccountFiscalPositionSet, account m.AccountAccountSet) m.AccountAccountSet {
			for _, pos := range rs.Accounts().Records() {
				if pos.AccountSrc().Equals(account) {
					return pos.AccountDest()
				}
			}
			return account
		})

	h.AccountFiscalPosition().Methods().MapAccounts().DeclareMethod(
		`MapAccounts Receive a dictionary having accounts in values and try to replace those accounts accordingly to the fiscal position.`,
		func(rs m.AccountFiscalPositionSet, accounts map[string]m.AccountAccountSet) map[string]m.AccountAccountSet {

			refDict := make(map[int64]m.AccountAccountSet)
			for _, line := range rs.Accounts().Records() {
				refDict[line.AccountSrc().ID()] = line.AccountDest()
			}
			for key, acc := range accounts {
				if val, ok := refDict[acc.ID()]; ok {
					accounts[key] = val
				}
			}
			return accounts
		})

	h.AccountFiscalPosition().Methods().OnchangeCountry().DeclareMethod(
		`OnchangeCountryId`,
		func(rs m.AccountFiscalPositionSet) m.AccountFiscalPositionData {
			data := h.AccountFiscalPosition().NewData()
			if rs.Country().IsNotEmpty() {
				data.
					SetZipFrom("0").
					SetZipTo("0").
					SetCountryGroup(h.CountryGroup().NewSet(rs.Env())).
					SetStates(h.CountryState().NewSet(rs.Env())).
					SetStatesCount(rs.Country().States().Len())
			}
			return data
		})

	h.AccountFiscalPosition().Methods().OnchangeCountryGroup().DeclareMethod(
		`OnchangeCountryGroupId`,
		func(rs m.AccountFiscalPositionSet) m.AccountFiscalPositionData {
			data := h.AccountFiscalPosition().NewData()
			if rs.Country().IsNotEmpty() {
				data.
					SetZipFrom("0").
					SetZipTo("0").
					SetCountry(h.Country().NewSet(rs.Env())).
					SetStates(h.CountryState().NewSet(rs.Env()))
			}
			return data
		})

	h.AccountFiscalPosition().Methods().GetFposByRegion().DeclareMethod(
		`GetFposByRegion`,
		func(rs m.AccountFiscalPositionSet, country m.CountrySet, state m.CountryStateSet, zipCode string,
			vatRequired bool) m.AccountFiscalPositionSet {

			if country.IsEmpty() {
				return h.AccountFiscalPosition().NewSet(rs.Env())
			}
			baseCond := q.AccountFiscalPosition().AutoApply().Equals(true).
				And().VatRequired().Equals(vatRequired)
			if id := rs.Env().Context().GetInteger("force_company"); id != 0 {
				baseCond = baseCond.And().CompanyFilteredOn(q.Company().ID().Equals(id))
			}
			nullStateCond := q.AccountFiscalPosition().States().Equals(nil)
			stateCond := nullStateCond
			nullZipCond := q.AccountFiscalPosition().ZipFrom().Equals("0").And().ZipTo().Equals("0")
			zipCond := nullZipCond
			nullCountryCond := q.AccountFiscalPosition().Country().Equals(nil).
				And().CountryGroup().Equals(nil)

			if zipCode == "" {
				zipCode = "0"
			}
			if zipCode != "0" {
				zipCond = q.AccountFiscalPosition().ZipFrom().LowerOrEqual(zipCode).
					And().ZipTo().GreaterOrEqual(zipCode)
			}
			if state.IsNotEmpty() {
				stateCond = q.AccountFiscalPosition().States().Equals(state)
			}

			CondCountry := baseCond.And().Country().Equals(country)
			CondGroup := baseCond.And().CountryGroupFilteredOn(q.CountryGroup().Countries().Equals(country))

			// Build domain to search records with exact matching criteria
			fpos := h.AccountFiscalPosition().NewSet(rs.Env()).Search(CondCountry.AndCond(stateCond).AndCond(zipCond)).Limit(1)
			// return records that fit the most the criteria, and fallback on less specific fiscal positions if any can be found
			if fpos.IsEmpty() && state.IsNotEmpty() {
				fpos = h.AccountFiscalPosition().NewSet(rs.Env()).Search(CondCountry.AndCond(nullStateCond).AndCond(zipCond)).Limit(1)
			}
			if fpos.IsEmpty() && zipCode != "0" {
				fpos = h.AccountFiscalPosition().NewSet(rs.Env()).Search(CondCountry.AndCond(stateCond).AndCond(nullZipCond)).Limit(1)
			}
			if fpos.IsEmpty() && state.IsNotEmpty() && zipCode != "0" {
				fpos = h.AccountFiscalPosition().NewSet(rs.Env()).Search(CondCountry.AndCond(nullStateCond).AndCond(nullZipCond)).Limit(1)
			}

			// fallback: country group with no state/zip range
			if fpos.IsEmpty() {
				fpos = h.AccountFiscalPosition().NewSet(rs.Env()).Search(CondGroup.AndCond(nullStateCond).AndCond(nullZipCond)).Limit(1)
			}
			if fpos.IsEmpty() {
				// Fallback on catchall (no country, no group)
				fpos = h.AccountFiscalPosition().NewSet(rs.Env()).Search(baseCond.AndCond(nullCountryCond)).Limit(1)
			}
			if fpos.IsEmpty() {
				return h.AccountFiscalPosition().NewSet(rs.Env())
			}
			return fpos
		})

	h.AccountFiscalPosition().Methods().GetFiscalPosition().DeclareMethod(
		`GetFiscalPosition`,
		func(rs m.AccountFiscalPositionSet, partner, delivery m.PartnerSet) m.AccountFiscalPositionSet {
			if partner.IsEmpty() {
				return h.AccountFiscalPosition().NewSet(rs.Env())
			}

			// if no delivery use invoicing
			if delivery.IsEmpty() {
				delivery = partner
			}

			// partner manually set fiscal position always win
			if val := h.AccountFiscalPosition().Coalesce(delivery.PropertyAccountPosition(), partner.PropertyAccountPosition()); val.IsNotEmpty() {
				return val
			}

			// First search only matching VAT positions
			vatRequired := false
			if partner.VAT() != "" {
				vatRequired = true
			}
			fp := rs.GetFposByRegion(delivery.Country(), delivery.State(), delivery.Zip(), vatRequired)

			// Then if VAT required found no match, try positions that do not require it
			if fp.IsEmpty() && vatRequired {
				fp = rs.GetFposByRegion(delivery.Country(), delivery.State(), delivery.Zip(), false)
			}
			if fp.IsEmpty() {
				return h.AccountFiscalPosition().NewSet(rs.Env())
			}
			return fp
		})

	h.AccountFiscalPositionTax().DeclareModel()
	h.AccountFiscalPositionTax().AddFields(map[string]models.FieldDefinition{
		"Position": models.Many2OneField{
			String:        "Fiscal Position",
			RelationModel: h.AccountFiscalPosition(),
			Required:      true,
			OnDelete:      models.Cascade},
		"TaxSrc": models.Many2OneField{
			String:        "Tax on Product",
			RelationModel: h.AccountTax(),
			Required:      true},
		"TaxDest": models.Many2OneField{
			String:        "Tax to Apply",
			RelationModel: h.AccountTax()},
	})

	h.AccountFiscalPositionTax().AddSQLConstraint("tax_src_dest_uniq",
		"unique (position_id, tax_src_id, tax_dest_id)",
		"A tax fiscal position could be defined only once time on same taxes.")

	h.AccountFiscalPositionTax().Methods().NameGet().Extend("",
		func(rs m.AccountFiscalPositionTaxSet) string {
			return rs.Position().DisplayName()
		})

	h.AccountFiscalPositionAccount().DeclareModel()
	h.AccountFiscalPositionAccount().AddFields(map[string]models.FieldDefinition{
		"Position": models.Many2OneField{
			String:        "Fiscal Position",
			RelationModel: h.AccountFiscalPosition(),
			Required:      true,
			OnDelete:      models.Cascade},
		"AccountSrc": models.Many2OneField{
			String:        "Account on Product",
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Required:      true},
		"AccountDest": models.Many2OneField{
			String:        "Account to Use Instead",
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().Deprecated().Equals(false),
			Required:      true},
	})

	h.AccountFiscalPositionAccount().AddSQLConstraint("account_src_dest_uniq",
		"unique (position_id, account_src_id, account_dest_id)",
		"An account fiscal position could be defined only once time on same accounts.")

	h.AccountFiscalPositionAccount().Methods().NameGet().Extend("",
		func(rs m.AccountFiscalPositionAccountSet) string {
			return rs.Position().DisplayName()
		})

	h.Partner().AddFields(map[string]models.FieldDefinition{
		"Credit": models.FloatField{
			String:  "Total Receivable",
			Compute: h.Partner().Methods().ComputeCreditDebit(), /*Search: "_credit_search"*/
			Help:    "Total amount this customer owes you."},
		"Debit": models.FloatField{
			String:  "Total Payable",
			Compute: h.Partner().Methods().ComputeCreditDebit(), /* Search: "_debit_search"*/
			Help:    "Total amount you have to pay to this vendor."},
		"DebitLimit": models.FloatField{
			String: "Payable Limit"},
		"TotalInvoiced": models.FloatField{
			Compute: h.Partner().Methods().ComputeTotalInvoiced(),
			InvisibleFunc: func(env models.Environment) (bool, models.Conditioner) {
				return !security.Registry.HasMembership(env.Uid(), GroupAccountInvoice), nil
			}},
		"Currency": models.Many2OneField{
			String:        "Currency",
			RelationModel: h.Currency(),
			Compute:       h.Partner().Methods().ComputeCurrency(),
			Help:          "Utility field to express amount currency"},
		"ContractsCount": models.IntegerField{
			String:  "Contracts",
			Compute: h.Partner().Methods().ComputeJournalItemCount(),
			GoType:  new(int)},
		"JournalItemCount": models.IntegerField{
			String:  "Journal Items",
			Compute: h.Partner().Methods().ComputeJournalItemCount(),
			GoType:  new(int)},
		"IssuedTotal": models.FloatField{
			String:  "Journal Items",
			Compute: h.Partner().Methods().ComputeIssuedTotal()},
		"PropertyAccountPayable": models.Many2OneField{
			String:        "Account Payable",
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().InternalType().Equals("payable").And().Deprecated().Equals(false),
			Help:          "This account will be used instead of the default one as the payable account for the current partner",
			Contexts:      base.CompanyDependent,
			Default: func(env models.Environment) interface{} {
				return h.AccountAccount().NewSet(env).GetDefaultAccountFromChart("PropertyAccountPayable")
			},
		},
		"PropertyAccountReceivable": models.Many2OneField{
			String:        "Account Receivable",
			RelationModel: h.AccountAccount(),
			Filter:        q.AccountAccount().InternalType().Equals("receivable").And().Deprecated().Equals(false),
			Help:          "This account will be used instead of the default one as the receivable account for the current partner",
			Contexts:      base.CompanyDependent,
			Default: func(env models.Environment) interface{} {
				return h.AccountAccount().NewSet(env).GetDefaultAccountFromChart("PropertyAccountReceivable")
			},
		},
		"PropertyAccountPosition": models.Many2OneField{
			String:        "Fiscal Position",
			RelationModel: h.AccountFiscalPosition(),
			Help:          "The fiscal position will determine taxes and accounts used for the partner.",
			Contexts:      base.CompanyDependent,
		},
		"PropertyPaymentTerm": models.Many2OneField{
			String:        "Customer Payment Terms",
			RelationModel: h.AccountPaymentTerm(),
			Help:          "This payment term will be used instead of the default one for sale orders and customer invoices",
			Contexts:      base.CompanyDependent,
		},
		"PropertySupplierPaymentTerm": models.Many2OneField{
			String:        "Vendor Payment Terms",
			RelationModel: h.AccountPaymentTerm(),
			Help:          "This payment term will be used instead of the default one for purchase orders and vendor bills",
			Contexts:      base.CompanyDependent,
		},
		"RefCompanies": models.One2ManyField{
			String:        "Companies that refers to partner",
			RelationModel: h.Company(),
			ReverseFK:     "Partner",
			JSON:          "ref_company_ids"},
		"HasUnreconciledEntries": models.BooleanField{
			Compute: h.Partner().Methods().ComputeHasUnreconciledEntries(),
			Help: `The partner has at least one unreconciled debit and credit
since last time the invoices & payments matching was performed.`},
		"LastTimeEntriesChecked": models.DateTimeField{
			String:   "Latest Invoices & Payments Matching Date",
			ReadOnly: true,
			NoCopy:   true,
			Help: `Last time the invoices & payments matching was performed for this partner.
It is set either if there\'s not at least an unreconciled debit and an unreconciled
credit or if you click the "Done" button.`},
		"Invoices": models.One2ManyField{
			RelationModel: h.AccountInvoice(),
			ReverseFK:     "Partner",
			JSON:          "invoice_ids",
			ReadOnly:      true},
		"Contracts": models.One2ManyField{
			RelationModel: h.AccountAnalyticAccount(),
			ReverseFK:     "Partner",
			JSON:          "contract_ids",
			ReadOnly:      true},
		"BankAccountCount": models.IntegerField{
			String:  "Bank",
			Compute: h.Partner().Methods().ComputeBankCount()},
		"Trust": models.SelectionField{
			String: "Degree of trust you have in this debtor", Selection: types.Selection{
				"good":   "Good Debtor",
				"normal": "Normal Debtor",
				"bad":    "Bad Debtor"},
			Default:  models.DefaultValue("normal"),
			Contexts: base.CompanyDependent},
		"InvoiceWarn": models.SelectionField{
			Selection: base.WarningMessage,
			String:    "Invoice",
			Help:      base.WarningHelp,
			Required:  true,
			Default:   models.DefaultValue("no-message")},
		"InvoiceWarnMsg": models.TextField{
			String: "Message for Invoice"},
	})

	h.Partner().Methods().ComputeCreditDebit().DeclareMethod(
		`CreditDebitGet`,
		func(rs m.PartnerSet) m.PartnerData {
			type tDest struct {
				typ string
				val float64
			}
			var whereClause string
			var whereParams []interface{}
			var out tDest

			whereClause, whereParams = h.AccountMoveLine().NewSet(rs.Env()).QueryGet(q.AccountMoveLineCondition{})
			whereParams = append([]interface{}{rs.Ids()}, whereParams...)
			if whereClause != "" {
				whereClause = "AND " + whereClause
			}
			rs.Env().Cr().Get(&out, `SELECT act.type, SUM(account_move_line.amount_residual)
			                FROM account_move_line
			                LEFT JOIN account_account a ON (account_move_line.account_id=a.id)
			                LEFT JOIN account_account_type act ON (a.user_type_id=act.id)
			                WHERE act.type IN ('receivable','payable')
			                AND account_move_line.partner_id IN %s
			                AND account_move_line.reconciled IS FALSE
			                `+whereClause+`
			                GROUP BY account_move_line.partner_id, act.type
							LIMIT 1
			                `, whereParams)

			partner := h.Partner().NewData()
			if out.typ == "receivable" {
				partner.SetCredit(out.val)
			} else if out.typ == "payable" {
				partner.SetDebit(-out.val)
			}
			return partner
		})

	h.Partner().Methods().AssetDifferenceSearch().DeclareMethod(
		`AssetDifferenceSearch`,
		func(rs m.PartnerSet, accountType string, op operator.Operator, operand float64) q.PartnerCondition {
			if !strutils.IsIn(string(op), "<", "=", ">", ">=", "<=") {
				return q.PartnerCondition{}
			}
			op.IsMulti()

			sign := 1
			if accountType == "payable" {
				sign = -1
			}

			var res struct{ ids []int64 }
			rs.Env().Cr().Select(&res, `
			      SELECT partner.id
			      FROM res_partner partner
			      LEFT JOIN account_move_line aml ON aml.partner_id = partner.id
			      RIGHT JOIN account_account acc ON aml.account_id = acc.id
			      WHERE acc.internal_type = %s
			        AND NOT acc.deprecated
			      GROUP BY partner.id
			      HAVING %s * COALESCE(SUM(aml.amount_residual), 0) `+string(op)+` %s`,
				accountType, sign, operand)
			if len(res.ids) == 0 {
				return q.Partner().ID().Equals(0)
			}
			return q.Partner().ID().In(res.ids)
		})

	h.Partner().Methods().CreditSearch().DeclareMethod(
		`CreditSearch returns the condition to search on partners credits.`,
		func(rs m.PartnerSet, op operator.Operator, operand interface{}) q.PartnerCondition {
			return rs.AssetDifferenceSearch("receivable", op, operand.(float64))
		})

	h.Partner().Methods().DebitSearch().DeclareMethod(
		`DebitSearch returns the condition to search on partners debits.`,
		func(rs m.PartnerSet, op operator.Operator, operand interface{}) q.PartnerCondition {
			return rs.AssetDifferenceSearch("payable", op, operand.(float64))
		})

	h.Partner().Methods().ComputeTotalInvoiced().DeclareMethod(
		`InvoiceTotal`,
		func(rs m.PartnerSet) m.PartnerData {
			data := h.Partner().NewData()
			if rs.IsEmpty() {
				return data.SetTotalInvoiced(0.0)
			}

			currentUser := h.User().NewSet(rs.Env()).CurrentUser()
			allPartners := h.Partner().NewSet(rs.Env()).WithContext("active_test", false).Search(q.Partner().ID().ChildOf(rs.ID()))

			condition := q.AccountInvoiceReport().Partner().In(allPartners).
				And().State().NotIn([]string{"draft", "cancel"}).
				And().Company().Equals(currentUser.Company()).
				And().Type().In([]string{"out_invoice", "out_refund"})

			aggs := h.AccountInvoiceReport().Search(rs.Env(), condition).
				GroupBy(h.AccountInvoiceReport().Fields().Partner()).
				Aggregates(
					h.AccountInvoiceReport().Fields().Partner(),
					h.AccountInvoiceReport().Fields().PriceTotal())
			if len(aggs) > 0 {
				data.SetTotalInvoiced(aggs[0].Values().PriceTotal())
			}
			return data
		})

	h.Partner().Methods().ComputeJournalItemCount().DeclareMethod(
		`ComputeJournalItemCount`,
		func(rs m.PartnerSet) m.PartnerData {
			data := h.Partner().NewData()

			data.SetJournalItemCount(h.AccountMoveLine().Search(rs.Env(), q.AccountMoveLine().Partner().Equals(rs)).Len())
			data.SetContractsCount(h.AccountAnalyticAccount().Search(rs.Env(), q.AccountAnalyticAccount().Partner().Equals(rs)).Len())
			return data
		})

	h.Partner().Methods().GetFollowupLinesDomain().DeclareMethod(
		`GetFollowupLinesDomain`,
		func(rs m.PartnerSet, date dates.Date, overdueOnly, onlyUnblocked bool) q.AccountMoveLineCondition {
			domain := q.AccountMoveLine().
				Reconciled().Equals(false).
				And().AccountFilteredOn(q.AccountAccount().Deprecated().Equals(false).
				And().InternalType().Equals("receivable")).
				AndCond(q.AccountMoveLine().Debit().NotEquals(0).
					Or().Credit().NotEquals(0)).
				And().Company().Equals(h.User().NewSet(rs.Env()).CurrentUser().Company())

			if onlyUnblocked {
				domain = domain.And().Blocked().Equals(false)
			}
			if rs.IsNotEmpty() {
				if rs.Env().Context().GetBool("exclude_given_ids") {
					domain = domain.And().Partner().NotIn(rs)
				} else {
					domain = domain.And().Partner().In(rs)
				}
			}
			// adding the overdue lines
			if overdueOnly {
				domain = domain.AndCond(q.AccountMoveLine().DateMaturity().NotEquals(dates.Date{}).
					And().DateMaturity().Lower(date).
					OrCond(q.AccountMoveLine().DateMaturity().Equals(dates.Date{}).
						And().Date().Lower(date)))
			}
			return domain
		})

	h.Partner().Methods().ComputeIssuedTotal().DeclareMethod(
		`ComputeIssuedTotal Returns the issued total as will be displayed on partner view`,
		func(rs m.PartnerSet) m.PartnerData {
			today := dates.Today()
			domain := rs.GetFollowupLinesDomain(today, true, false)
			domain = domain.And().Partner().Equals(rs)
			total := 0.0
			for _, aml := range h.AccountMoveLine().Search(rs.Env(), domain).Records() {
				total += aml.AmountResidual()
			}
			return h.Partner().NewData().SetIssuedTotal(total)
		})

	h.Partner().Methods().ComputeHasUnreconciledEntries().DeclareMethod(
		`ComputeHasUnreconciledEntries`,
		func(rs m.PartnerSet) m.PartnerData {
			// Avoid useless work if has_unreconciled_entries is not relevant for this partner
			if !rs.Active() || !rs.IsCompany() && rs.Parent().IsNotEmpty() {
				return h.Partner().NewData()
			}
			var dest []interface{}
			rs.Env().Cr().Select(&dest, `
					  SELECT 1 FROM(
			              SELECT
			                  p.last_time_entries_checked AS last_time_entries_checked,
			                  MAX(l.write_date) AS max_date
			              FROM
			                  account_move_line l
			                  RIGHT JOIN account_account a ON (a.id = l.account_id)
			                  RIGHT JOIN res_partner p ON (l.partner_id = p.id)
			              WHERE
			                  p.id = %s
			                  AND EXISTS (
			                      SELECT 1
			                      FROM account_move_line l
			                      WHERE l.account_id = a.id
			                      AND l.partner_id = p.id
			                      AND l.amount_residual > 0
			                  )
			                  AND EXISTS (
			                      SELECT 1
			                      FROM account_move_line l
			                      WHERE l.account_id = a.id
			                      AND l.partner_id = p.id
			                      AND l.amount_residual < 0
			                  )
			              GROUP BY p.last_time_entries_checked
			          ) as s
			          WHERE (last_time_entries_checked IS NULL OR max_date > last_time_entries_checked)
			      `, rs.ID())
			return h.Partner().NewData().SetHasUnreconciledEntries(len(dest) != 0)
		})

	h.Partner().Methods().MarkAsReconciled().DeclareMethod(
		`MarkAsReconciled`,
		func(rs m.PartnerSet) bool {
			h.AccountPartialReconcile().NewSet(rs.Env()).CheckAccessRights(webdata.CheckAccessRightsArgs{"write", true})
			return rs.Sudo().Write(h.Partner().NewData().SetLastTimeEntriesChecked(dates.Now()))
		})

	h.Partner().Methods().ComputeCurrency().DeclareMethod(
		`GetCompanyCurrency`,
		func(rs m.PartnerSet) m.PartnerData {
			if rs.Company().IsNotEmpty() {
				return h.Partner().NewData().SetCurrency(rs.Sudo().Company().Currency())
			} else {
				return h.Partner().NewData().SetCurrency(h.User().NewSet(rs.Env()).CurrentUser().Company().Currency())
			}
		})

	h.Partner().Methods().ComputeBankCount().DeclareMethod(
		`ComputeBankCount`,
		func(rs m.PartnerSet) m.PartnerData {
			bankData := h.BankAccount().NewSet(rs.Env()).ReadGroup(webdata.ReadGroupParams{
				Domain:  q.BankAccount().Partner().In(rs).Serialize(),
				Fields:  []string{"Partner"},
				GroupBy: []string{"Partner"},
			})
			var value int64 = 0
			var val interface{}
			val, ok := bankData[0].Get("PartnerCount", h.BankAccount().Model)
			if ok {
				value = val.(int64)
			}
			return h.Partner().NewData().SetBankAccountCount(value)
		})

	h.Partner().Methods().FindAccountingPartner().DeclareMethod(
		`FindAccountingPartner finds the partner for which the accounting entries will be created`,
		func(rs m.PartnerSet, partner m.PartnerSet) m.PartnerSet {
			return partner.CommercialPartner()
		})

	h.Partner().Methods().CommercialFields().Extend("",
		func(rs m.PartnerSet) []models.FieldNamer {
			return append(rs.Super().CommercialFields(), models.ConvertToFieldNameSlice([]string{
				`DebitLimit`, `PropertyAccountPayable`, `PropertyAccountReceivable`, `PropertyAccountPosition`,
				`PropertyPaymentTerm`, `PropertySupplierPaymentTerm`, `LastTimeEntriesChecked`})...)
		})

	h.Partner().Methods().OpenPartnerHistory().DeclareMethod(
		`OpenPartnerHistory returns an action that display invoices/refunds made for the given partners.`,
		func(rs m.PartnerSet) *actions.Action {
			action := actions.Registry.GetById("account_action_invoice_refund_out_tree")
			// FIXME
			//cond := domains.ParseDomain(action.Domain)
			//cond = cond.AndCond(q.Partner().Parent().ChildOf(rs).Condition)
			//action.Domain = domains.Domain(cond.Serialize()).String()
			return action
		})
}
