module github.com/hexya-addons/account

replace github.com/hexya-erp/pool => /home/ray/workspace/go/project/pool

replace github.com/hexya-addons/account => /home/ray/workspace/go/account

replace github.com/hexya-erp/hexya => /home/ray/workspace/go/hexya

require (
	github.com/hexya-addons/analytic v0.0.8
	github.com/hexya-addons/base v0.0.7
	github.com/hexya-addons/decimalPrecision v0.0.7
	github.com/hexya-addons/web v0.0.7
	github.com/hexya-erp/hexya v0.0.9
	github.com/hexya-erp/pool v1.0.0 // indirect
	github.com/jmoiron/sqlx v1.2.0
)
