// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-addons/base"
	"github.com/hexya-erp/hexya/src/models/security"
	"github.com/hexya-erp/pool/h"
)

var (
	// GroupAccountInvoice is the group of users allowed to bill customers
	GroupAccountInvoice *security.Group
	// GroupAccountUser is the group of accountant users
	GroupAccountUser *security.Group
	// GroupAccountManager is the group of accounting/finance managers
	GroupAccountManager *security.Group
)

func init() {
	GroupAccountInvoice = security.Registry.NewGroup("account_group_account_invoice", "Billing", base.GroupUser)
	GroupAccountUser = security.Registry.NewGroup("account_group_account_user", "Accountant", GroupAccountInvoice)
	GroupAccountManager = security.Registry.NewGroup("account_group_account_manager", "Adviser", GroupAccountUser)

	h.ProductProduct().Methods().Load().AllowGroup(GroupAccountUser)
	h.ProductProduct().Methods().AllowAllToGroup(GroupAccountManager)
	h.ProductTemplate().Methods().AllowAllToGroup(GroupAccountManager)
	h.ProductPriceHistory().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountPaymentTerm().Methods().Load().AllowGroup(GroupAccountUser)
	h.AccountPaymentTermLine().Methods().Load().AllowGroup(GroupAccountUser)
	h.AccountAccountType().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountAccountType().Methods().Load().AllowGroup(GroupAccountUser)
	h.AccountTax().Methods().Load().AllowGroup(base.GroupUser)
	h.AccountAccount().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountAccount().Methods().Load().AllowGroup(GroupAccountUser)
	h.AccountAccount().Methods().Load().AllowGroup(base.GroupUser)
	h.AccountAccount().Methods().Load().AllowGroup(base.GroupPartnerManager)
	h.AccountTax().Methods().Load().AllowGroup(GroupAccountInvoice)
	h.AccountAccountTemplate().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountChartTemplate().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountTaxTemplate().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountBankStatement().Methods().Load().AllowGroup(GroupAccountInvoice)
	h.AccountBankStatementLine().Methods().Load().AllowGroup(GroupAccountInvoice)
	h.AccountBankStatement().Methods().AllowAllToGroup(GroupAccountUser)
	h.AccountBankStatementLine().Methods().AllowAllToGroup(GroupAccountUser)
	h.AccountAnalyticLine().Methods().Load().AllowGroup(GroupAccountManager)
	h.AccountAnalyticAccount().Methods().Load().AllowGroup(base.GroupUser)
	h.AccountInvoice().Methods().AllowAllToGroup(GroupAccountInvoice)
	h.AccountInvoiceLine().Methods().AllowAllToGroup(GroupAccountInvoice)
	h.AccountInvoiceTax().Methods().AllowAllToGroup(GroupAccountInvoice)
	h.AccountMove().Methods().AllowAllToGroup(GroupAccountInvoice)
	h.AccountMoveLine().Methods().AllowAllToGroup(GroupAccountInvoice)
	h.AccountPaymentTerm().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountPaymentTermLine().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountTax().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountJournal().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountJournal().Methods().Load().AllowGroup(GroupAccountUser)
	h.AccountJournal().Methods().Load().AllowGroup(GroupAccountInvoice)
	h.AccountInvoice().Methods().AllowAllToGroup(GroupAccountInvoice)
	h.Currency().Methods().AllowAllToGroup(GroupAccountManager)
	h.CurrencyRate().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountInvoice().Methods().Load().AllowGroup(base.GroupUser)
	h.AccountInvoiceLine().Methods().Load().AllowGroup(base.GroupUser)
	h.AccountPaymentTerm().Methods().Load().AllowGroup(base.GroupUser)
	h.AccountPaymentTermLine().Methods().Load().AllowGroup(base.GroupUser)
	h.AccountFiscalPosition().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountFiscalPositionTax().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountFiscalPositionAccount().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountFiscalPosition().Methods().Load().AllowGroup(base.GroupUser)
	h.AccountFiscalPositionTax().Methods().Load().AllowGroup(base.GroupUser)
	h.AccountFiscalPositionAccount().Methods().Load().AllowGroup(base.GroupUser)
	h.AccountFiscalPositionTemplate().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountFiscalPositionTaxTemplate().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountFiscalPositionAccountTemplate().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountInvoiceReport().Methods().Load().AllowGroup(GroupAccountUser)
	h.AccountInvoiceReport().Methods().AllowAllToGroup(GroupAccountManager)
	h.AccountInvoiceReport().Methods().Load().AllowGroup(GroupAccountInvoice)
	h.Partner().Methods().Load().AllowGroup(GroupAccountManager)
	h.AccountInvoice().Methods().Load().AllowGroup(GroupAccountUser)
	h.AccountMoveLine().Methods().Load().AllowGroup(GroupAccountManager)
	h.AccountMove().Methods().Load().AllowGroup(GroupAccountManager)
	h.AccountInvoiceTax().Methods().Load().AllowGroup(GroupAccountUser)
	h.AccountAnalyticLine().Methods().AllowAllToGroup(GroupAccountInvoice)
	h.AccountInvoiceLine().Methods().Load().AllowGroup(GroupAccountUser)
	h.AccountAccount().Methods().Load().AllowGroup(GroupAccountInvoice)
	h.AccountAnalyticAccount().Methods().AllowAllToGroup(GroupAccountUser)
	h.AccountAccountType().Methods().Load().AllowGroup(GroupAccountInvoice)
	h.AccountFinancialReport().Methods().AllowAllToGroup(GroupAccountUser)
	h.AccountFinancialReport().Methods().Load().AllowGroup(GroupAccountInvoice)
	h.AccountAccountTag().Methods().Load().AllowGroup(GroupAccountUser)
	h.AccountAccountTag().Methods().Load().AllowGroup(GroupAccountInvoice)
	h.AccountReconcileModel().Methods().AllowAllToGroup(GroupAccountUser)
	h.AccountReconcileModelTemplate().Methods().AllowAllToGroup(GroupAccountUser)
	h.AccountPartialReconcile().Methods().AllowAllToGroup(GroupAccountInvoice)
	h.AccountPartialReconcile().Methods().AllowAllToGroup(GroupAccountUser)
	h.AccountFullReconcile().Methods().AllowAllToGroup(GroupAccountInvoice)
	h.AccountFullReconcile().Methods().AllowAllToGroup(GroupAccountUser)
	h.AccountPaymentMethod().Methods().AllowAllToGroup(GroupAccountInvoice)
	h.AccountPayment().Methods().AllowAllToGroup(GroupAccountInvoice)
	h.AccountBankStatementCashbox().Methods().AllowAllToGroup(GroupAccountUser)
	h.AccountCashboxLine().Methods().AllowAllToGroup(GroupAccountUser)
	h.AccountTaxGroup().Methods().Load().AllowGroup(base.GroupUser)
	h.AccountTaxGroup().Methods().Load().AllowGroup(GroupAccountInvoice)
	h.AccountTaxGroup().Methods().AllowAllToGroup(GroupAccountManager)
}
