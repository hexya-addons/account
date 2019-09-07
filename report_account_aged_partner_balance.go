// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"fmt"
	"strconv"

	"github.com/hexya-addons/account/accounttypes"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
)

func init() {

	h.ReportAccountReportAgedpartnerbalance().DeclareTransientModel()
	h.ReportAccountReportAgedpartnerbalance().Methods().GetPartnerMoveLines().DeclareMethod(
		`GetPartnerMoveLines`,
		func(rs m.ReportAccountReportAgedpartnerbalanceSet, accountType []string, dateFrom dates.Date, targetMove string,
			periodLength int) ([]accounttypes.AgedBalanceReportValues, []float64, map[int64][]accounttypes.AgedBalanceReportLine) {
			// This method can receive the context key 'include_nullified_amount' {Boolean}
			// Do an invoice and a payment and unreconcile. The amount will be nullified
			// By default, the partner wouldn't appear in this report.
			// The context key allow it to appear

			periods := make(map[string]accounttypes.AgedBalancePeriod)
			start := dateFrom
			for i := 4; i >= 0; i-- {
				stop := start.AddDate(0, 0, -periodLength)
				ps := dates.Date{}
				if i != 0 {
					ps = stop
				}
				name := fmt.Sprintf("+%d", 4*periodLength)
				if i != 0 {
					name = fmt.Sprintf("%d-%d", (5-(i+1))*periodLength, (5-i)*periodLength)
				}
				periods[strconv.Itoa(i)] = accounttypes.AgedBalancePeriod{
					Name:  name,
					Start: ps,
					Stop:  start,
				}
				start = stop.AddDate(0, 0, 1)
			}

			var res []accounttypes.AgedBalanceReportValues

			userCompany := h.User().NewSet(rs.Env()).CurrentUser().Company()
			userCurrency := userCompany.Currency()
			companies := userCompany
			if rs.Env().Context().HasKey("company_ids") {
				companies = h.Company().Browse(rs.Env(), rs.Env().Context().GetIntegerSlice("company_ids"))
			}
			moveState := []string{"draft", "post"}
			if targetMove == "posted" {
				moveState = []string{"posted"}
			}

			argsList := []interface{}{moveState, accountType}
			// build the reconciliation clause to see which partner needs to be printed
			reconciliationClause := `(l.reconciled IS FALSE)`
			var reconciledAfterDate []struct {
				DebitMoveID  int64 `db:"debit_move_id"`
				CreditMoveID int64 `db:"credit_move_id"`
			}
			rs.Env().Cr().Select(&reconciledAfterDate,
				`SELECT debit_move_id, credit_move_id FROM account_partial_reconcile where create_date > ?`,
				dateFrom)
			var reconciledIds []int64
			for _, r := range reconciledAfterDate {
				reconciledIds = append(reconciledIds, r.CreditMoveID, r.DebitMoveID)
			}

			if len(reconciledAfterDate) > 0 {
				reconciliationClause = `(l.reconciled IS FALSE or l.id in (?)`
				argsList = append(argsList, reconciledIds)
			}
			argsList = append(argsList, dateFrom, companies.Ids())

			query := fmt.Sprintf(`SELECT DISTINCT l.partner_id, UPPER(partner.name) as name
					FROM account_move_line AS l
						LEFT JOIN partner on l.partner_id = partner.id
						LEFT JOIN account_account acc ON l.account_id = acc.id
						LEFT JOIN account_move am ON l.move_id = am.id
						LEFT JOIN account_account_type aat ON acc.user_type_id = aat.id
					WHERE (am.state IN (?))
						AND (aat.type IN (?))
						AND %s
						AND (am.date <= ?)
						AND acc.company_id IN (?)
					ORDER BY UPPER(partner.name)`, reconciliationClause)
			var partners []struct {
				PartnerID int64  `db:"partner_id"`
				Name      string `db:"name"`
			}
			rs.Env().Cr().Select(&partners, query, argsList...)

			total := []float64{0, 0, 0, 0, 0, 0, 0}
			lines := make(map[int64][]accounttypes.AgedBalanceReportLine)

			// Build a string like (1,2,3) for easy use in SQL query
			var partnerIds []int64
			for _, p := range partners {
				if p.PartnerID != 0 {
					partnerIds = append(partnerIds, p.PartnerID)
				}
			}
			if len(partnerIds) == 0 {
				return []accounttypes.AgedBalanceReportValues{}, []float64{}, map[int64][]accounttypes.AgedBalanceReportLine{}
			}

			// This dictionary will store the not due amount of all partners
			undueAmounts := make(map[int64]float64)
			query = `SELECT l.id
					FROM account_move_line AS l
						LEFT JOIN account_account acc ON l.account_id = acc.id
						LEFT JOIN account_move am ON l.move_id = am.id
						LEFT JOIN account_account_type aat ON acc.user_type_id = aat.id
					WHERE (am.state IN (?))
						AND (aat.type IN (?))
						AND (COALESCE(l.date_maturity,am.date) > ?)
						AND ((l.partner_id IN (?)) OR (l.partner_id IS NULL))
					AND (am.date <= ?)
					AND acc.company_id IN (?)`
			var amlIds []int64
			rs.Env().Cr().Select(&amlIds, query, moveState, accountType, dateFrom, partnerIds, dateFrom, companies.Ids())

			for _, line := range h.AccountMoveLine().Browse(rs.Env(), amlIds).Records() {
				partnerID := line.Partner().ID()
				if _, exists := undueAmounts[partnerID]; !exists {
					undueAmounts[partnerID] = 0
				}
				lineAmount := line.Company().Currency().WithContext("date", dateFrom).Compute(line.Balance(), userCurrency, true)
				if userCurrency.IsZero(lineAmount) {
					continue
				}
				for _, partialLine := range line.MatchedDebits().Records() {
					if partialLine.CreateDate().ToDate().LowerEqual(dateFrom) {
						lineAmount += partialLine.Company().Currency().WithContext("date", dateFrom).Compute(partialLine.Amount(), userCurrency, true)
					}
				}
				for _, partialLine := range line.MatchedCredits().Records() {
					if partialLine.CreateDate().ToDate().LowerEqual(dateFrom) {
						lineAmount -= partialLine.Company().Currency().WithContext("date", dateFrom).Compute(partialLine.Amount(), userCurrency, true)
					}
				}
				if !h.User().NewSet(rs.Env()).CurrentUser().Company().Currency().IsZero(lineAmount) {
					undueAmounts[partnerID] += lineAmount
					lines[partnerID] = append(lines[partnerID], accounttypes.AgedBalanceReportLine{
						LineID: line.ID(),
						Amount: lineAmount,
						Period: 6,
					})
				}
			}

			// Use one query per period and store results in history (a list variable)
			// Each history will contain: history[1] = {'<partner_id>': <partner_debit-credit>}
			history := make([]map[int64]float64, 5)
			for i := 0; i < 5; i++ {
				period := periods[strconv.Itoa(i)]
				argsList = []interface{}{moveState, accountType, partnerIds}
				datesQuery := `(COALESCE(l.date_maturity, am.date)`

				switch {
				case !period.Start.IsZero() && !period.Stop.IsZero():
					datesQuery += ` BETWEEN ? AND ?)`
					argsList = append(argsList, period.Start, period.Stop)
				case !period.Start.IsZero():
					datesQuery += ` >= ?)`
					argsList = append(argsList)
				case !period.Stop.IsZero():
					datesQuery += ` <= ?)`
					argsList = append(argsList, period.Stop)
				}
				argsList = append(argsList, dateFrom, companies.Ids())
				query := fmt.Sprintf(`SELECT l.id
						FROM account_move_line AS l
							LEFT JOIN account_account acc ON l.account_id = acc.id
							LEFT JOIN account_move am ON l.move_id = am.id
							LEFT JOIN account_account_type aat ON acc.user_type_id = aat.id
						WHERE (am.state IN (?))
							AND (aat.type IN (?))
							AND ((l.partner_id IN (?)) OR (l.partner_id IS NULL))
							AND %s
						AND (am.date <= ?)
						AND acc.company_id IN (?)`, datesQuery)
				var amlIds []int64
				partnersAmount := make(map[int64]float64)
				rs.Env().Cr().Select(&amlIds, query, argsList...)
				for _, line := range h.AccountMoveLine().Browse(rs.Env(), amlIds).Records() {
					partnerID := line.Partner().ID()
					if _, exists := partnersAmount[partnerID]; !exists {
						partnersAmount[partnerID] = 0
					}
					lineAmount := line.Company().Currency().WithContext("date", dateFrom).Compute(line.Balance(), userCurrency, true)
					if userCurrency.IsZero(lineAmount) {
						continue
					}
					for _, partialLine := range line.MatchedDebits().Records() {
						if partialLine.CreateDate().ToDate().LowerEqual(dateFrom) {
							lineAmount += partialLine.Company().Currency().WithContext("date", dateFrom).Compute(partialLine.Amount(), userCurrency, true)
						}
					}
					for _, partialLine := range line.MatchedCredits().Records() {
						if partialLine.CreateDate().ToDate().LowerEqual(dateFrom) {
							lineAmount -= partialLine.Company().Currency().WithContext("date", dateFrom).Compute(partialLine.Amount(), userCurrency, true)
						}
					}
					if !h.User().NewSet(rs.Env()).CurrentUser().Company().Currency().IsZero(lineAmount) {
						partnersAmount[partnerID] += lineAmount
						lines[partnerID] = append(lines[partnerID], accounttypes.AgedBalanceReportLine{
							LineID: line.ID(),
							Amount: lineAmount,
							Period: i + 1,
						})
					}
				}
				history[i] = partnersAmount
			}

			for _, partner := range partners {
				var (
					atLeastOneAmount bool
					values           accounttypes.AgedBalanceReportValues
					undueAmount      float64
				)
				if _, exists := undueAmounts[partner.PartnerID]; exists {
					// Making sure this partner actually was found by the query
					undueAmount = undueAmounts[partner.PartnerID]
				}
				total[6] += undueAmount
				values.Direction = undueAmount
				if !h.User().NewSet(rs.Env()).CurrentUser().Company().Currency().IsZero(values.Direction) {
					atLeastOneAmount = true
				}

				for i := 0; i < 5; i++ {
					var during float64
					if _, exists := history[i][partner.PartnerID]; exists {
						during = history[i][partner.PartnerID]
					}
					// Adding counter
					total[i] += during
					values.Values[i] = during
					if !h.User().NewSet(rs.Env()).CurrentUser().Company().Currency().IsZero(values.Values[i]) {
						atLeastOneAmount = true
					}
				}
				values.Total = values.Direction
				for _, val := range values.Values {
					values.Total += val
				}
				// Add for total
				total[5] += values.Total
				values.PartnerID = partner.PartnerID
				values.Name = rs.T("Unknown Partner")
				if partner.PartnerID != 0 {
					browsedPartner := h.Partner().BrowseOne(rs.Env(), partner.PartnerID)
					values.Name = browsedPartner.Name()
					if len(values.Name) >= 45 {
						values.Name = values.Name[:40] + "..."
					}
					values.Trust = true
				}

				_, linesHasPartner := lines[partner.PartnerID]
				if atLeastOneAmount || (rs.Env().Context().GetBool("include_nullified_amount") && linesHasPartner) {
					res = append(res, values)
				}
			}
			return res, total, lines
		})

	h.ReportAccountReportAgedpartnerbalance().Methods().RenderHtml().DeclareMethod(
		`RenderHtml`,
		func(rs m.ReportAccountReportAgedpartnerbalanceSet, args struct {
			Docids interface{}
			Data   interface{}
		}) {
			//@api.model
			/*def render_html(self, docids, data=None):
			  if not data.get('form') or not self.env.context.get('active_model') or not self.env.context.get('active_id'):
			      raise UserError(_("Form content is missing, this report cannot be printed."))

			  total = []
			  model = self.env.context.get('active_model')
			  docs = self.env[model].browse(self.env.context.get('active_id'))

			  target_move = data['form'].get('target_move', 'all')
			  date_from = data['form'].get('date_from', time.strftime('%Y-%m-%d'))

			  if data['form']['result_selection'] == 'customer':
			      account_type = ['receivable']
			  elif data['form']['result_selection'] == 'supplier':
			      account_type = ['payable']
			  else:
			      account_type = ['payable', 'receivable']

			  movelines, total, dummy = self._get_partner_move_lines(account_type, date_from, target_move, data['form']['period_length'])
			  docargs = {
			      'doc_ids': self.ids,
			      'doc_model': model,
			      'data': data['form'],
			      'docs': docs,
			      'time': time,
			      'get_partner_lines': movelines,
			      'get_direction': total,
			  }
			  return self.env['report'].render('account.report_agedpartnerbalance', docargs)
			*/
		})

}
