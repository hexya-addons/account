// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-addons/web/domains"
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/hexya/src/models"
	"github.com/hexya-erp/hexya/src/models/fieldtype"
	"github.com/hexya-erp/hexya/src/models/types"
	"github.com/hexya-erp/hexya/src/models/types/dates"
	"github.com/hexya-erp/hexya/src/tools/strutils"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
)

func init() {

	h.AccountInvoiceRefund().DeclareTransientModel()
	h.AccountInvoiceRefund().Methods().GetReason().DeclareMethod(
		`GetReason`,
		func(rs m.AccountInvoiceRefundSet) string {
			context := rs.Env().Context()
			activeID := context.GetInteger("active_id")
			if activeID != 0 {
				return h.AccountInvoice().BrowseOne(rs.Env(), activeID).Name()
			}
			return ""
		})
	h.AccountInvoiceRefund().AddFields(map[string]models.FieldDefinition{
		"DateInvoice": models.DateField{
			String:   "Refund Date",
			Required: true,
			Default:  models.DefaultValue(dates.Today())},
		"Date": models.DateField{
			String: "Accounting Date"},
		"Description": models.CharField{
			String:   "Reason",
			Required: true,
			Default: func(env models.Environment) interface{} {
				return h.AccountInvoiceRefund().NewSet(env).GetReason()
			}},
		"RefundOnly": models.BooleanField{
			String:  "Technical field to hide filter_refund in case invoice is partially paid",
			Compute: h.AccountInvoiceRefund().Methods().GetRefundOnly()},
		"FilterRefund": models.SelectionField{
			String: "Refund Method",
			Selection: types.Selection{
				"refund": "Create a draft refund",
				"cancel": "Cancel: create refund and reconcile",
				"modify": "Modify: create refund reconcile and create a new draft invoice"},
			Default:  models.DefaultValue("refund"),
			Required: true,
			Help:     "Refund base on this type. You can not Modify and Cancel if the invoice is already reconciled"},
	})
	h.AccountInvoiceRefund().Methods().GetRefundOnly().DeclareMethod(
		`GetRefundOnly`,
		func(rs m.AccountInvoiceRefundSet) m.AccountInvoiceRefundData {
			data := h.AccountInvoiceRefund().NewData()
			invoice := h.AccountInvoice().BrowseOne(rs.Env(), rs.Env().Context().GetInteger("active_id"))
			if invoice.PaymentMoveLines().IsNotEmpty() && invoice.State() != "paid" {
				data.SetRefundOnly(true)
			} else {
				data.SetRefundOnly(false)
			}
			return data
		})

	h.AccountInvoiceRefund().Methods().ComputeRefund().DeclareMethod(
		`ComputeRefund`,
		func(rs m.AccountInvoiceRefundSet, mode string) *actions.Action {
			if mode == "" {
				mode = "refund"
			}
			var xmlID string
			var createdInv []int64

			for _, form := range rs.Records() {
				createdInv = []int64{}
				for _, inv := range h.AccountInvoice().Browse(rs.Env(), rs.Env().Context().GetIntegerSlice("active_ids")).Records() {
					if strutils.IsIn(inv.State(), "draft", "proforma2", "cancel") {
						panic(rs.T(`Cannot refund draft/proforma/cancelled invoice.`))
					}
					if inv.Reconciled() && strutils.IsIn(mode, "cancel", "modify") {
						panic(rs.T(`Cannot refund invoice which is already reconciled, invoice should be unreconciled first. You can only refund this invoice.`))
					}

					date := form.Date()
					description := form.Description()
					if description == "" {
						description = inv.Name()
					}
					refund := inv.Refund(form.DateInvoice(), date, description, inv.Journal())
					createdInv = append(createdInv, refund.ID())

					if strutils.IsIn(mode, "cancel", "modify") {
						movelines := inv.Move().Lines()
						toReconcileLines := h.AccountMoveLine().NewSet(rs.Env())
						for _, line := range movelines.Records() {
							if line.Account().Equals(inv.Account()) {
								toReconcileLines = toReconcileLines.Union(line)
							}
							if line.Reconciled() {
								line.RemoveMoveReconcile()
							}
						}
						refund.ActionInvoiceOpen()
						for _, tmpline := range refund.Move().Lines().Records() {
							if tmpline.Account().Equals(inv.Account()) {
								toReconcileLines = toReconcileLines.Union(tmpline)
								toReconcileLines.Filtered(func(set m.AccountMoveLineSet) bool {
									return set.Reconciled() == false
								})
							}
						}
						if mode == "modify" {
							invoice := inv.First().Copy()
							invoice.UnsetID().
								UnsetNumber().
								SetDateInvoice(form.DateInvoice()).
								SetState("draft").
								SetDate(date)
							for _, field := range h.AccountInvoice().NewSet(rs.Env()).GetRefundCommonFields() {
								if h.AccountInvoice().NewSet(rs.Env()).FieldGet(field).Type == fieldtype.Many2One {
									if val, ok := invoice.Get(field.String()); ok {
										invoice.Set(field.String(), val.(models.RecordSet).Collection().Records()[0])
									}
								}
							}
							invRefund := h.AccountInvoice().Create(rs.Env(), invoice)
							if invRefund.PaymentTerm().IsNotEmpty() {
								invRefund.OnchangePaymentTermDateInvoice()
							}
							createdInv = append(createdInv, invRefund.ID())
						}
					}
					if strutils.IsIn(inv.Type(), "out_refund", "out_invoice") {
						xmlID = "action_invoice_tree1"
					} else if strutils.IsIn(inv.Type(), "in_refund", "in_invoice") {
						xmlID = "action_invoice_tree2"
					}
					// Put the reason in the chatter
					/* TODO module mail NYI
						subject = _("Invoice refund")
					  	body = description
					  	refund.message_post(body=body, subject=subject)
					*/
				}
			}
			if xmlID != "" {
				result := actions.Registry.GetById("account." + xmlID)
				invoiceDomain := *domains.ParseString(result.Domain)
				invoiceDomain = append(invoiceDomain, []interface{}{"id", "in", createdInv})
				result.Domain = invoiceDomain.String()
				return result
			}
			return nil
		})
	h.AccountInvoiceRefund().Methods().InvoiceRefund().DeclareMethod(
		`InvoiceRefund`,
		func(rs m.AccountInvoiceRefundSet) *actions.Action {
			return rs.ComputeRefund(rs.FilterRefund())
		})

}
