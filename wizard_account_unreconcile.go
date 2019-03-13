// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
)

func init() {

	h.AccountUnreconcile().DeclareTransientModel()
	h.AccountUnreconcile().Methods().TransUnrec().DeclareMethod(
		`TransUnrec`,
		func(rs m.AccountUnreconcileSet) *actions.Action {
			if ids := rs.Env().Context().GetIntegerSlice("active_ids"); len(ids) > 0 {
				h.AccountMoveLine().Browse(rs.Env(), ids).RemoveMoveReconcile()
			}
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

}
