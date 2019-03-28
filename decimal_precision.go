package account

import (
	"github.com/hexya-addons/decimalPrecision"
	"github.com/hexya-erp/hexya/src/tools/nbutils"
)

func init() {
	decimalPrecision.Precisions["Payment Terms"] = nbutils.Digits{Precision: 6, Scale: 2}
}
