// Copyright 2017 NDP Systèmes. All Rights Reserved.
// See LICENSE file for full licensing details.

package account

import (
	"github.com/hexya-erp/hexya/src/actions"
	"github.com/hexya-erp/pool/h"
	"github.com/hexya-erp/pool/m"
)

func init() {

	h.ValidateAccountMove().DeclareTransientModel()
	h.ValidateAccountMove().Methods().ValidateMove().DeclareMethod(
		`ValidateMove`,
		func(rs m.ValidateAccountMoveSet) *actions.Action {
			context := rs.Env().Context()
			moves := h.AccountMove().Browse(rs.Env(), context.GetIntegerSlice("active_ids"))
			moveToPost := h.AccountMove().NewSet(rs.Env())
			for _, move := range moves.Records() {
				if move.State() == "draft" {
					moveToPost = moves.Union(move)
				}
			}
			if moveToPost.IsEmpty() {
				panic(rs.T(`There is no journal items in draft state to post.`))
			}
			moveToPost.Post()
			return &actions.Action{
				Type: actions.ActionCloseWindow,
			}
		})

}
