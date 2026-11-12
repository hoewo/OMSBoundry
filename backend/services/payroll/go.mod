module omsbizfoundry/payroll

go 1.26.0

toolchain go1.26.8

require (
	github.com/jackc/pgx/v5 v5.11.0
	omsbizfoundry/attendance-summary-contract v0.0.0
	omsbizfoundry/workload-identity v0.0.0
)

replace omsbizfoundry/attendance-summary-contract => ../../../packages/attendance-summary-contract

replace omsbizfoundry/workload-identity => ../../../packages/workload-identity

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/sync v0.17.0 // indirect
	golang.org/x/text v0.29.0 // indirect
)
